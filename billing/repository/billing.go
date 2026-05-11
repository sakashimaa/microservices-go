package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/billing-microservice/billing/domain"
)

var (
	ErrIdempotencyConflict = errors.New("idempotency key already exists")
	ErrTransactionNotFound = errors.New("transaction not found")
)

type BillingRepository interface {
	TopUpAccountTx(ctx context.Context, tx pgx.Tx, params domain.TopUpAccountParams) (string, error)
	InsertTransactionTx(ctx context.Context, tx pgx.Tx, params domain.InsertTransactionParams) error
	FindByIdempotencyKey(ctx context.Context, params domain.FindByIdempotencyKeyParams) (*domain.Transaction, error)
}

func NewBillingRepo(db *pgxpool.Pool) BillingRepository {
	return &BillingPGRepo{
		db: db,
	}
}

type BillingPGRepo struct {
	db *pgxpool.Pool
}

func (r *BillingPGRepo) FindByIdempotencyKey(ctx context.Context, params domain.FindByIdempotencyKeyParams) (*domain.Transaction, error) {
	var res domain.Transaction
	query := `
		SELECT id, account_id, amount, type, created_at, idempotency_key
		FROM transactions
		WHERE idempotency_key = $1
	`

	if err := r.db.QueryRow(
		ctx,
		query,
		params.IdempotencyKey,
	).Scan(
		&res.Id,
		&res.AccountId,
		&res.Amount,
		&res.Type,
		&res.CreatedAt,
		&res.IdempotencyKey,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}

		return nil, fmt.Errorf("find transaction by idempotency key: %w", err)
	}

	return &res, nil
}

func (r *BillingPGRepo) InsertTransactionTx(ctx context.Context, tx pgx.Tx, params domain.InsertTransactionParams) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO transactions (account_id, amount, type, idempotency_key)
		VALUES ($1, $2, 'deposit', $3)
	`, params.AccountId, params.Amount, params.IdempotencyKey)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				if pgErr.ConstraintName == "transactions_idempotency_key_uniq" {
					return ErrIdempotencyConflict
				}
			}
		}

		return fmt.Errorf("insert transaction failed: %w", err)
	}

	return nil
}

func (r *BillingPGRepo) TopUpAccountTx(ctx context.Context, tx pgx.Tx, params domain.TopUpAccountParams) (string, error) {
	var accountID string
	err := tx.QueryRow(ctx, `
		INSERT INTO accounts (user_id, balance)
		VALUES ($1, $2)
		ON CONFLICT(user_id) DO UPDATE
		SET balance = accounts.balance + $2
		RETURNING id
	`, params.UserId, params.Amount).Scan(&accountID)
	if err != nil {
		return "", fmt.Errorf("failed to upsert account: %w", err)
	}

	return accountID, nil
}
