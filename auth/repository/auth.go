package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/billing-microservice/auth/domain"
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
)

type AuthRepository interface {
	Register(ctx context.Context, data domain.CreateUserParams) (*domain.User, error)
}

func NewAuthRepository(db *pgxpool.Pool) AuthRepository {
	return &AuthPGRepo{
		db: db,
	}
}

type AuthPGRepo struct {
	db *pgxpool.Pool
}

func (r *AuthPGRepo) Register(ctx context.Context, data domain.CreateUserParams) (*domain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func(ctx context.Context) {
		if err = tx.Rollback(ctx); err != nil {
			fmt.Printf("failed to rollback register transaction auth repo: %v\n", err)
		}
	}(ctx)

	insertUserQuery := `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (email) DO NOTHING
		RETURNING id, email, password_hash, created_at
	`

	var result domain.User
	if err = tx.QueryRow(ctx, insertUserQuery, data.Email, data.PasswordHash).Scan(&result.Id, &result.Email, &result.Password, &result.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserAlreadyExists
		}

		return nil, fmt.Errorf("insert user: %w", err)
	}

	return &result, nil
}
