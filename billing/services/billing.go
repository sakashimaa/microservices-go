package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAccountNotFound   = errors.New("account not found")
)

type BillingService interface {
	Deposit(ctx context.Context, userID int64, amount int64) error
	Withdraw(ctx context.Context, userID int64, amount int64) error
}

type BillingPGService struct {
	db *pgxpool.Pool
}

func (s *BillingPGService) Deposit(ctx context.Context, userID int64, amount int64) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx failed: %w", err)
	}
	defer tx.Rollback(ctx)

	var accountID string

	err = tx.QueryRow(ctx, `
		INSERT INTO accounts (user_id, balance)
		VALUES ($1, $2)
		ON CONFLICT(user_id) DO UPDATE
		SET balance = accounts.balance + $2
		RETURNING id
	`, userID, amount).Scan(&accountID)
	if err != nil {
		return fmt.Errorf("failed to upsert account: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO transactions (account_id, amount, type)
		VALUES ($1, $2, 'deposit')
	`, accountID, amount)
	if err != nil {
		return fmt.Errorf("insert transaction failed: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *BillingPGService) Withdraw(ctx context.Context, userID int64, amount int64) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx failed: %w", err)
	}
	defer tx.Rollback(ctx)

	var accountID string
	var balance int64

	err = tx.QueryRow(ctx, `
		SELECT id, balance
		FROM accounts
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&accountID, &balance)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return ErrAccountNotFound
		}

		return fmt.Errorf("failed to query account: %w", err)
	}

	if balance < amount {
		return ErrInsufficientFunds
	}

	_, err = tx.Exec(ctx, `
		UPDATE accounts
		SET balance = balance - $1
		WHERE id = $2
	`, amount, accountID)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO transactions (account_id, amount, type)
		VALUES ($1, $2, 'withdrawal')
	`, accountID, amount)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	return tx.Commit(ctx)
}

func NewBillingPGService(db *pgxpool.Pool) BillingService {
	return &BillingPGService{
		db: db,
	}
}
