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
	ErrUserNotFound      = errors.New("user not found")
	ErrTokenNotFound     = errors.New("token not found")
)

type AuthRepository interface {
	Register(ctx context.Context, data domain.CreateUserParams) (*domain.User, error)
	GetUserByEmail(ctx context.Context, data domain.GetUserByEmailParams) (*domain.User, error)
	SaveRefresh(ctx context.Context, data domain.SaveTokenParams) error
	SaveRefreshTx(ctx context.Context, tx pgx.Tx, data domain.SaveTokenParams) error
	BeginTx(ctx context.Context) (pgx.Tx, error)
	GetTokenForUpdateTx(ctx context.Context, tx pgx.Tx, hashStr string) (*domain.Token, error)
	RevokeAllUserTokensTx(ctx context.Context, tx pgx.Tx, userId string) error
	MarkTokenAsConsumedTx(ctx context.Context, tx pgx.Tx, oldHashStr string, newHashStr string) error
	GetUserById(ctx context.Context, userId string) (*domain.User, error)
	DeleteExpiredTokens(ctx context.Context, limit int) error
}

func NewAuthRepository(db *pgxpool.Pool) AuthRepository {
	return &AuthPGRepo{
		db: db,
	}
}

type AuthPGRepo struct {
	db *pgxpool.Pool
}

func (r *AuthPGRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

func (r *AuthPGRepo) DeleteExpiredTokens(ctx context.Context, limit int) error {
	query := `
		DELETE FROM refresh_tokens
		WHERE id IN (
			SELECT id
			FROM refresh_tokens
			WHERE expires_at < NOW()
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		);
	`

	_, err := r.db.Exec(ctx, query, limit)
	if err != nil {
		return fmt.Errorf("delete expired tokens: %w", err)
	}

	return nil
}

func (r *AuthPGRepo) GetUserById(ctx context.Context, userId string) (*domain.User, error) {
	query := `
		SELECT id, email, created_at
		FROM users
		WHERE id = $1
	`

	var res domain.User
	if err := r.db.QueryRow(
		ctx,
		query,
		userId,
	).Scan(
		&res.Id,
		&res.Email,
		&res.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	return &res, nil
}

func (r *AuthPGRepo) SaveRefreshTx(ctx context.Context, tx pgx.Tx, data domain.SaveTokenParams) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`

	_, err := tx.Exec(ctx, query, data.UserId, data.Token, data.ExpiresAt)
	if err != nil {
		return fmt.Errorf("save refresh tx failed: %w", err)
	}

	return nil
}

func (r *AuthPGRepo) MarkTokenAsConsumedTx(ctx context.Context, tx pgx.Tx, oldHashStr string, newHashStr string) error {
	query := `
		UPDATE refresh_tokens
		SET consumed_at = NOW(),
			revoked = true,
			revoked_by = $1
		WHERE token_hash = $2
	`

	_, err := tx.Exec(ctx, query, newHashStr, oldHashStr)
	if err != nil {
		return fmt.Errorf("mark token as consumed: %w", err)
	}

	return nil
}

func (r *AuthPGRepo) RevokeAllUserTokensTx(ctx context.Context, tx pgx.Tx, userId string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = true
		WHERE user_id = $1
			AND revoked = false
			AND expires_at > NOW()
	`

	cmdTag, err := tx.Exec(ctx, query, userId)
	if err != nil {
		return fmt.Errorf("revoke all user tokens: %w", err)
	}

	fmt.Printf("revoked %d sessions for user %s due to security breach", cmdTag.RowsAffected(), userId)
	return nil
}

func (r *AuthPGRepo) GetTokenForUpdateTx(ctx context.Context, tx pgx.Tx, hashStr string) (*domain.Token, error) {
	query := `
		SELECT id, user_id, expires_at, consumed_at, revoked, revoked_by
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`

	var result domain.Token
	if err := tx.QueryRow(
		ctx,
		query,
		hashStr,
	).Scan(
		&result.Id,
		&result.UserId,
		&result.ExpiresAt,
		&result.ConsumedAt,
		&result.Revoked,
		&result.RevokedBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}

		return nil, fmt.Errorf("get token for update: %w", err)
	}

	return &result, nil
}

func (r *AuthPGRepo) SaveRefresh(ctx context.Context, data domain.SaveTokenParams) error {
	query := `
		INSERT INTO refresh_tokens (
			user_id, 
			token_hash, 
			expires_at
		) VALUES (
			$1,
			$2,
			$3
		)
	`

	_, err := r.db.Exec(
		ctx,
		query,
		data.UserId,
		data.Token,
		data.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("insert token: %w", err)
	}

	return nil
}

func (r *AuthPGRepo) GetUserByEmail(ctx context.Context, data domain.GetUserByEmailParams) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`

	var result domain.User
	if err := r.db.QueryRow(
		ctx,
		query,
		data.Email,
	).Scan(&result.Id, &result.Email, &result.Password, &result.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return &result, nil
}

func (r *AuthPGRepo) Register(ctx context.Context, data domain.CreateUserParams) (*domain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(err, pgx.ErrTxClosed) {
			fmt.Printf("rollback register transaction: %v\n", err)
		}
	}()

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

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &result, nil
}
