package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/billing-microservice/billing/domain"
	"github.com/sakashimaa/billing-microservice/billing/repository"
)

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAccountNotFound   = errors.New("account not found")
)

type BillingService interface {
	Deposit(ctx context.Context, req domain.DepositRequest) error
	Withdraw(ctx context.Context, req domain.WithdrawRequest) error
}

func NewBillingService(db *pgxpool.Pool, repo repository.BillingRepository) BillingService {
	return &BillingPGService{
		db:   db,
		repo: repo,
	}
}

type BillingPGService struct {
	db   *pgxpool.Pool
	repo repository.BillingRepository
}

func (s *BillingPGService) Deposit(ctx context.Context, req domain.DepositRequest) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx failed: %w", err)
	}
	defer tx.Rollback(ctx)

	accountId, err := s.repo.TopUpAccountTx(ctx, tx, domain.TopUpAccountParams{
		Amount: req.Amount,
		UserId: req.UserId,
	})
	if err != nil {
		return fmt.Errorf("deposit: %w", err)
	}

	err = s.repo.InsertTransactionTx(ctx, tx, domain.InsertTransactionParams{
		AccountId:      accountId,
		Amount:         req.Amount,
		Type:           "deposit",
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		if errors.Is(err, repository.ErrIdempotencyConflict) {
			if err := tx.Rollback(ctx); err != nil {
				return fmt.Errorf("deposit transaction rollback: %w", err)
			}

			_, fetchErr := s.repo.FindByIdempotencyKey(ctx, domain.FindByIdempotencyKeyParams{
				IdempotencyKey: req.IdempotencyKey,
			})
			if fetchErr != nil {
				return fmt.Errorf("failed to fetch idempotent transaction: %w", err)
			}

			return nil
		}

		return fmt.Errorf("insert transaction failed: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *BillingPGService) Withdraw(ctx context.Context, req domain.WithdrawRequest) error {
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
	`, req.UserId).Scan(&accountID, &balance)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return ErrAccountNotFound
		}

		return fmt.Errorf("failed to query account: %w", err)
	}

	if balance < req.Amount {
		return ErrInsufficientFunds
	}

	_, err = tx.Exec(ctx, `
		UPDATE accounts
		SET balance = balance - $1
		WHERE id = $2
	`, req.Amount, accountID)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO transactions (account_id, amount, type)
		VALUES ($1, $2, 'withdrawal')
	`, accountID, req.Amount)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	return tx.Commit(ctx)
}
