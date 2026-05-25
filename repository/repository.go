package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
)

type TestRepository interface {
	GetHello() string
	Init(ctx context.Context) error
	CreateDBTest(ctx context.Context, body string) (DBTestRecord, error)
	GetDBTestByID(ctx context.Context, id int64) (DBTestRecord, error)
	CreateUser(ctx context.Context, params CreateUserParams) (User, error)
	GetUserByLogin(ctx context.Context, login string) (User, error)
	Close() error
}

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

type testRepository struct {
	db *sql.DB
}

func NewTestRepository(db *sql.DB) TestRepository {
	return &testRepository{db: db}
}

func (r *testRepository) GetHello() string {
	return "Hello!"
}

func (r *testRepository) Init(ctx context.Context) error {
	queries := []string{
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
			status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'sold')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS db_test (
			id BIGSERIAL PRIMARY KEY,
			body TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
	}

	for _, q := range queries {
		if _, err := r.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("init db: %w", err)
		}
	}

	return nil
}

func (r *testRepository) CreateDBTest(ctx context.Context, body string) (DBTestRecord, error) {
	const q = `INSERT INTO db_test (body) VALUES ($1) RETURNING id, body, created_at`

	var record DBTestRecord
	if err := r.db.QueryRowContext(ctx, q, body).Scan(&record.ID, &record.Body, &record.CreatedAt); err != nil {
		return DBTestRecord{}, fmt.Errorf("create db_test record: %w", err)
	}

	return record, nil
}

func (r *testRepository) GetDBTestByID(ctx context.Context, id int64) (DBTestRecord, error) {
	const q = `SELECT id, body, created_at FROM db_test WHERE id = $1`

	var record DBTestRecord
	if err := r.db.QueryRowContext(ctx, q, id).Scan(&record.ID, &record.Body, &record.CreatedAt); err != nil {
		return DBTestRecord{}, fmt.Errorf("get db_test record: %w", err)
	}

	return record, nil
}

func (r *testRepository) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	const q = `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email, password_hash, created_at
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

func (r *testRepository) GetUserByLogin(ctx context.Context, login string) (User, error) {
	const q = `
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE LOWER(username) = LOWER($1) OR LOWER(email) = LOWER($1)
	`

	var user User
	err := r.db.QueryRowContext(ctx, q, login).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get user by login: %w", err)
	}

	return user, nil
}

func (r *testRepository) Close() error {
	if r.db == nil {
		return nil
	}
	return r.db.Close()
}

func isUniqueViolation(err error) bool {
	message := err.Error()
	return strings.Contains(message, "duplicate key value violates unique constraint") ||
		strings.Contains(message, "SQLSTATE 23505")
}
