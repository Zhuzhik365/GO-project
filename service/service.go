package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"go-project/repository"
	"strings"
	"time"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid login or password")
)

type TestService interface {
	GetMessage() string
	SaveDBTest(ctx context.Context, body string) (repository.DBTestRecord, error)
	RegisterUser(ctx context.Context, username, email, password string) (repository.User, error)
	LoginUser(ctx context.Context, login, password string) (AuthResponse, error)
}

type AuthResponse struct {
	Token     string          `json:"token"`
	ExpiresAt time.Time       `json:"expires_at"`
	User      repository.User `json:"user"`
}

type testService struct {
	repo      repository.TestRepository
	jwtSecret []byte
	tokenTTL  time.Duration
}

func NewTestService(repo repository.TestRepository, jwtSecret string) TestService {
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	return &testService{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  24 * time.Hour,
	}
}

func (s *testService) GetMessage() string {
	return s.repo.GetHello()
}

func (s *testService) SaveDBTest(ctx context.Context, body string) (repository.DBTestRecord, error) {
	record, err := s.repo.CreateDBTest(ctx, body)
	if err != nil {
		return repository.DBTestRecord{}, err
	}

	return s.repo.GetDBTestByID(ctx, record.ID)
}

func (s *testService) RegisterUser(ctx context.Context, username, email, password string) (repository.User, error) {
	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))

	if len(username) < 3 || len(email) < 5 || !strings.Contains(email, "@") || len(password) < 6 {
		return repository.User{}, fmt.Errorf("%w: username must be at least 3 chars, email must be valid, password must be at least 6 chars", ErrInvalidInput)
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return repository.User{}, err
	}

	user, err := s.repo.CreateUser(ctx, repository.CreateUserParams{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return repository.User{}, ErrUserAlreadyExists
		}
		return repository.User{}, err
	}

	return user, nil
}

func (s *testService) LoginUser(ctx context.Context, login, password string) (AuthResponse, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" || password == "" {
		return AuthResponse{}, ErrInvalidCredentials
	}

	user, err := s.repo.GetUserByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return AuthResponse{}, ErrInvalidCredentials
		}
		return AuthResponse{}, err
	}

	if !verifyPassword(user.PasswordHash, password) {
		return AuthResponse{}, ErrInvalidCredentials
	}

	expiresAt := time.Now().Add(s.tokenTTL)
	token, err := s.createJWT(user, expiresAt)
	if err != nil {
		return AuthResponse{}, err
	}

	user.PasswordHash = ""
	return AuthResponse{Token: token, ExpiresAt: expiresAt, User: user}, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := makePasswordHash(salt, password)
	return base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}

func verifyPassword(storedHash, password string) bool {
	parts := strings.Split(storedHash, "$")
	if len(parts) != 2 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	actualHash := makePasswordHash(salt, password)
	return hmac.Equal(expectedHash, actualHash)
}

func makePasswordHash(salt []byte, password string) []byte {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	return h.Sum(nil)
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

func (s *testService) createJWT(user repository.User, expiresAt time.Time) (string, error) {
	header := jwtHeader{Algorithm: "HS256", Type: "JWT"}
	payload := jwtPayload{
		UserID:   user.ID,
		Username: user.Username,
		IssuedAt: time.Now().Unix(),
		Expires:  expiresAt.Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	encoder := base64.RawURLEncoding
	unsignedToken := encoder.EncodeToString(headerJSON) + "." + encoder.EncodeToString(payloadJSON)

	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(unsignedToken))
	signature := encoder.EncodeToString(mac.Sum(nil))

	return unsignedToken + "." + signature, nil
}
