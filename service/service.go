package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-project/broker"
	"go-project/repository"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid login or password")
	ErrInvalidToken       = errors.New("invalid token")
)

type MessagePublisher interface {
	PublishAdCreated(ctx context.Context, msg broker.AdCreatedMessage) error
}

type Service interface {
	GetMessage() string
	CreateDBTest(ctx context.Context, body string) (repository.DBTestRecord, error)
	RegisterUser(ctx context.Context, username, email, password string) (repository.User, error)
	LoginUser(ctx context.Context, login, password string) (AuthResponse, error)
	ParseToken(token string) (int64, error)
	CreateAd(ctx context.Context, userID int64, title, description string, price float64) (repository.Ad, error)
	GetAdsByUserID(ctx context.Context, userID int64) ([]repository.Ad, error)
}

type AppService struct {
	repo      repository.Repository
	jwtSecret []byte
	publisher MessagePublisher
}

type AuthResponse struct {
	Token     string          `json:"token"`
	ExpiresAt time.Time       `json:"expires_at"`
	User      repository.User `json:"user"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type jwtPayload struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

func NewAppService(repo repository.Repository, jwtSecret string, publisher MessagePublisher) *AppService {
	return &AppService{repo: repo, jwtSecret: []byte(jwtSecret), publisher: publisher}
}

func (s *AppService) GetMessage() string {
	return s.repo.GetHello()
}

func (s *AppService) CreateDBTest(ctx context.Context, body string) (repository.DBTestRecord, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return repository.DBTestRecord{}, fmt.Errorf("%w: body is empty", ErrInvalidInput)
	}
	return s.repo.CreateDBTest(ctx, body)
}

func (s *AppService) RegisterUser(ctx context.Context, username, email, password string) (repository.User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)

	if username == "" || email == "" || password == "" {
		return repository.User{}, fmt.Errorf("%w: username, email and password are required", ErrInvalidInput)
	}
	if len(password) < 6 {
		return repository.User{}, fmt.Errorf("%w: password must be at least 6 characters", ErrInvalidInput)
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return repository.User{}, fmt.Errorf("hash password: %w", err)
	}

	return s.repo.CreateUser(ctx, repository.CreateUserParams{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	})
}

func (s *AppService) LoginUser(ctx context.Context, login, password string) (AuthResponse, error) {
	login = strings.TrimSpace(login)
	password = strings.TrimSpace(password)
	if login == "" || password == "" {
		return AuthResponse{}, fmt.Errorf("%w: login and password are required", ErrInvalidInput)
	}

	user, err := s.repo.GetUserByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return AuthResponse{}, ErrInvalidCredentials
		}
		return AuthResponse{}, err
	}

	if !verifyPassword(password, user.PasswordHash) {
		return AuthResponse{}, ErrInvalidCredentials
	}

	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	token, err := s.createJWT(user, expiresAt)
	if err != nil {
		return AuthResponse{}, err
	}

	return AuthResponse{Token: token, ExpiresAt: expiresAt, User: user}, nil
}

func (s *AppService) CreateAd(ctx context.Context, userID int64, title, description string, price float64) (repository.Ad, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)

	if userID <= 0 || title == "" || description == "" || price < 0 {
		return repository.Ad{}, fmt.Errorf("%w: invalid ad data", ErrInvalidInput)
	}

	ad, err := s.repo.CreateAd(ctx, userID, title, description, price, "pending")
	if err != nil {
		return repository.Ad{}, err
	}

	if s.publisher != nil {
		msg := broker.AdCreatedMessage{AdID: ad.ID, UserID: ad.UserID, Title: ad.Title, CreatedAt: ad.CreatedAt}
		if err := s.publisher.PublishAdCreated(ctx, msg); err != nil {
			return repository.Ad{}, fmt.Errorf("publish ad created event: %w", err)
		}
	}

	return ad, nil
}

func (s *AppService) GetAdsByUserID(ctx context.Context, userID int64) ([]repository.Ad, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}
	return s.repo.GetAdsByUserID(ctx, userID)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	saltHex := hex.EncodeToString(salt)
	return saltHex + "$" + makePasswordHash(password, saltHex), nil
}

func verifyPassword(password, storedHash string) bool {
	parts := strings.Split(storedHash, "$")
	if len(parts) != 2 {
		return false
	}
	saltHex, expected := parts[0], parts[1]
	actual := makePasswordHash(password, saltHex)
	return hmac.Equal([]byte(actual), []byte(expected))
}

func makePasswordHash(password, saltHex string) string {
	sum := sha256.Sum256([]byte(saltHex + password))
	return hex.EncodeToString(sum[:])
}

func (s *AppService) createJWT(user repository.User, expiresAt time.Time) (string, error) {
	header := jwtHeader{Algorithm: "HS256", Type: "JWT"}
	payload := jwtPayload{UserID: user.ID, Username: user.Username, IssuedAt: time.Now().UTC().Unix(), Expires: expiresAt.Unix()}

	encodedHeader, err := encodeJSON(header)
	if err != nil {
		return "", err
	}
	encodedPayload, err := encodeJSON(payload)
	if err != nil {
		return "", err
	}

	unsigned := encodedHeader + "." + encodedPayload
	signature := s.sign(unsigned)
	return unsigned + "." + signature, nil
}

func (s *AppService) ParseToken(token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, ErrInvalidToken
	}

	unsigned := parts[0] + "." + parts[1]
	expectedSignature := s.sign(unsigned)
	if !hmac.Equal([]byte(expectedSignature), []byte(parts[2])) {
		return 0, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, ErrInvalidToken
	}

	var payload jwtPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return 0, ErrInvalidToken
	}
	if payload.Expires < time.Now().UTC().Unix() {
		return 0, ErrInvalidToken
	}
	if payload.UserID <= 0 {
		return 0, ErrInvalidToken
	}
	return payload.UserID, nil
}

func encodeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *AppService) sign(unsigned string) string {
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
