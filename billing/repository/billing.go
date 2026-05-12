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
	ErrAccountNotFound     = errors.New("account not found")
)

type BillingRepository interface {
	TopUpAccountTx(ctx context.Context, tx pgx.Tx, params domain.TopUpAccountParams) (string, error)
	InsertTransactionTx(ctx context.Context, tx pgx.Tx, params domain.InsertTransactionParams) error
	GetAccountByUserIdTx(ctx context.Context, tx pgx.Tx, params domain.GetAccountByUserIdParams) (*domain.Account, error)
	FindByIdempotencyKey(ctx context.Context, params domain.FindByIdempotencyKeyParams) (*domain.Transaction, error)
	WithdrawAccountTx(ctx context.Context, tx pgx.Tx, params domain.WithdrawAccountParams) error
	InsertOutboxEventTx(ctx context.Context, tx pgx.Tx, eventType, aggregateId string, payload any) error
	QueryOutboxEventsTx(ctx context.Context, tx pgx.Tx, limit int) ([]*domain.OutboxEvent, error)
	MarkEventsAsProcessedTx(ctx context.Context, tx pgx.Tx, eventIds []int64) (int64, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

func NewBillingRepo(db *pgxpool.Pool) BillingRepository {
	return &BillingPGRepo{
		db: db,
	}
}

type BillingPGRepo struct {
	db *pgxpool.Pool
}

func (r *BillingPGRepo) MarkEventsAsProcessedTx(ctx context.Context, tx pgx.Tx, eventIds []int64) (int64, error) {
	query := `
		UPDATE outbox_events
		SET status = 'PROCESSED', processed_at = NOW()
		WHERE id = ANY($1)
	`

	cmdTag, err := tx.Exec(ctx, query, eventIds)
	if err != nil {
		return 0, fmt.Errorf("failed to mark events as processed: %w", err)
	}

	return cmdTag.RowsAffected(), nil
}

func (r *BillingPGRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("transaction start failed: %w", err)
	}

	return tx, nil
}

func (r *BillingPGRepo) QueryOutboxEventsTx(ctx context.Context, tx pgx.Tx, limit int) ([]*domain.OutboxEvent, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type,
				payload, status, error_text, processed_at
		FROM outbox_events
		WHERE status = 'PENDING'
		ORDER BY id ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query outbox: %w", err)
	}
	defer rows.Close()

	var res []*domain.OutboxEvent
	for rows.Next() {
		var e domain.OutboxEvent
		if err := rows.Scan(
			&e.Id,
			&e.AggregateType,
			&e.AggregateId,
			&e.EventType,
			&e.Payload,
			&e.Status,
			&e.ErrorText,
			&e.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		res = append(res, &e)
	}

	return res, nil
}

func (r *BillingPGRepo) InsertOutboxEventTx(ctx context.Context, tx pgx.Tx, eventType, aggregateId string, payload any) error {
	query := `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		VALUES ('account', $1, $2, $3)
	`

	_, err := tx.Exec(ctx, query, aggregateId, eventType, payload)
	if err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	return nil
}

func (r *BillingPGRepo) WithdrawAccountTx(ctx context.Context, tx pgx.Tx, params domain.WithdrawAccountParams) error {
	query := `
		UPDATE accounts
		SET balance = balance - $1
		WHERE id = $2
	`

	_, err := tx.Exec(ctx, query, params.Amount, params.Id)
	if err != nil {
		return fmt.Errorf("withdraw account: %w", err)
	}

	return nil
}

func (r *BillingPGRepo) GetAccountByUserIdTx(ctx context.Context, tx pgx.Tx, params domain.GetAccountByUserIdParams) (*domain.Account, error) {
	var res domain.Account
	query := `
		SELECT id, user_id, balance, created_at
		FROM accounts
		WHERE user_id = $1
		FOR UPDATE
	`

	err := tx.QueryRow(ctx, query, params.UserId).Scan(&res.Id, &res.UserId, &res.Balance, &res.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}

		return nil, fmt.Errorf("failed to query account: %w", err)
	}

	return &res, nil
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
		VALUES ($1, $2, $3, $4)
	`, params.AccountId, params.Amount, params.Type, params.IdempotencyKey)
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
