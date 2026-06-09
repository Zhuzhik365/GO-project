package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
	ErrAdNotFound        = errors.New("ad not found")
)

type DBTestRecord struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateUserParams struct {
	Username     string
	Email        string
	PasswordHash string
}

type Ad struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Repository interface {
	GetHello() string
	Init(ctx context.Context) error
	CreateDBTest(ctx context.Context, body string) (DBTestRecord, error)
	CreateUser(ctx context.Context, params CreateUserParams) (User, error)
	GetUserByLogin(ctx context.Context, login string) (User, error)
	CreateAd(ctx context.Context, userID int64, title, description string, price float64, status string) (Ad, error)
	GetAdsByUserID(ctx context.Context, userID int64) ([]Ad, error)
	UpdateAdStatus(ctx context.Context, adID int64, status string) error
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) GetHello() string {
	return "Hello!"
}

func (r *PostgresRepository) Init(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS db_test (
			id BIGSERIAL PRIMARY KEY,
			body TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			username VARCHAR(64) NOT NULL UNIQUE,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS ads (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			description TEXT NOT NULL,
			price NUMERIC(12, 2) NOT NULL CHECK (price >= 0),
			status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'sold')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`ALTER TABLE ads DROP CONSTRAINT IF EXISTS ads_status_check;`,
		`ALTER TABLE ads ADD CONSTRAINT ads_status_check CHECK (status IN ('pending', 'active', 'sold'));`,
		`ALTER TABLE ads ALTER COLUMN status SET DEFAULT 'pending';`,
		`CREATE INDEX IF NOT EXISTS idx_ads_user_id ON ads(user_id);`,
	}

	for _, q := range queries {
		if _, err := r.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("init database: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepository) CreateDBTest(ctx context.Context, body string) (DBTestRecord, error) {
	const q = `INSERT INTO db_test (body) VALUES ($1) RETURNING id, body, created_at;`

	var rec DBTestRecord
	if err := r.db.QueryRowContext(ctx, q, body).Scan(&rec.ID, &rec.Body, &rec.CreatedAt); err != nil {
		return DBTestRecord{}, fmt.Errorf("create db_test record: %w", err)
	}
	return rec, nil
}

func (r *PostgresRepository) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	const q = `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email, password_hash, created_at;
	`

	var user User
	err := r.db.QueryRowContext(ctx, q, params.Username, params.Email, params.PasswordHash).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUserAlreadyExists
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r *PostgresRepository) GetUserByLogin(ctx context.Context, login string) (User, error) {
	const q = `
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE LOWER(username) = LOWER($1) OR LOWER(email) = LOWER($1)
		LIMIT 1;
	`

	var user User
	err := r.db.QueryRowContext(ctx, q, login).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get user by login: %w", err)
	}
	return user, nil
}

func (r *PostgresRepository) CreateAd(ctx context.Context, userID int64, title, description string, price float64, status string) (Ad, error) {
	const q = `
		INSERT INTO ads (user_id, title, description, price, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, title, description, price, status, created_at, updated_at;
	`

	var ad Ad
	err := r.db.QueryRowContext(ctx, q, userID, title, description, price, status).Scan(
		&ad.ID,
		&ad.UserID,
		&ad.Title,
		&ad.Description,
		&ad.Price,
		&ad.Status,
		&ad.CreatedAt,
		&ad.UpdatedAt,
	)
	if err != nil {
		return Ad{}, fmt.Errorf("create ad: %w", err)
	}
	return ad, nil
}

func (r *PostgresRepository) GetAdsByUserID(ctx context.Context, userID int64) ([]Ad, error) {
	const q = `
		SELECT id, user_id, title, description, price, status, created_at, updated_at
		FROM ads
		WHERE user_id = $1
		ORDER BY id DESC;
	`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get ads by user id: %w", err)
	}
	defer rows.Close()

	ads := make([]Ad, 0)
	for rows.Next() {
		var ad Ad
		if err := rows.Scan(
			&ad.ID,
			&ad.UserID,
			&ad.Title,
			&ad.Description,
			&ad.Price,
			&ad.Status,
			&ad.CreatedAt,
			&ad.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ad: %w", err)
		}
		ads = append(ads, ad)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ads: %w", err)
	}
	return ads, nil
}

func (r *PostgresRepository) UpdateAdStatus(ctx context.Context, adID int64, status string) error {
	const q = `
		UPDATE ads
		SET status = $2, updated_at = NOW()
		WHERE id = $1;
	`

	res, err := r.db.ExecContext(ctx, q, adID, status)
	if err != nil {
		return fmt.Errorf("update ad status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return ErrAdNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
