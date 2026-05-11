package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
			if err := s.handleIdempotencyError(ctx, tx, req.IdempotencyKey); err != nil {
				return fmt.Errorf("idempotency error: %w", err)
			}

			return nil
		}

		return fmt.Errorf("insert transaction failed: %w", err)
	}

	payload := map[string]any{
		"user_id":   req.UserId,
		"amount":    req.Amount,
		"tx_type":   "deposit",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	err = s.repo.InsertOutboxEventTx(ctx, tx, "AccountToppedUp", accountId, payload)
	if err != nil {
		return fmt.Errorf("failed to write outbox: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *BillingPGService) Withdraw(ctx context.Context, req domain.WithdrawRequest) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx failed: %w", err)
	}
	defer tx.Rollback(ctx)

	res, err := s.repo.GetAccountByUserIdTx(ctx, tx, domain.GetAccountByUserIdParams{
		UserId: req.UserId,
	})
	if err != nil {
		return fmt.Errorf("withdraw: %w", err)
	}

	if res.Balance < req.Amount {
		return ErrInsufficientFunds
	}

	err = s.repo.WithdrawAccountTx(ctx, tx, domain.WithdrawAccountParams{
		Amount: req.Amount,
		Id:     res.Id,
	})
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	err = s.repo.InsertTransactionTx(ctx, tx, domain.InsertTransactionParams{
		AccountId:      res.Id,
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
		Type:           "withdrawal",
	})
	if err != nil {
		if errors.Is(err, repository.ErrIdempotencyConflict) {
			if err := s.handleIdempotencyError(ctx, tx, req.IdempotencyKey); err != nil {
				return fmt.Errorf("idempotency error: %w", err)
			}

			return nil
		}

		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *BillingPGService) handleIdempotencyError(ctx context.Context, tx pgx.Tx, idempotencyKey string) error {
	if err := tx.Rollback(ctx); err != nil {
		return fmt.Errorf("deposit transaction rollback: %w", err)
	}

	_, fetchErr := s.repo.FindByIdempotencyKey(ctx, domain.FindByIdempotencyKeyParams{
		IdempotencyKey: idempotencyKey,
	})
	if fetchErr != nil {
		return fmt.Errorf("failed to fetch idempotent transaction: %w", fetchErr)
	}

	return nil
}
