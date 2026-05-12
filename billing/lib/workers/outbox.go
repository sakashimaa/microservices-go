package workers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sakashimaa/billing-microservice/billing/lib/broker"
	"github.com/sakashimaa/billing-microservice/billing/repository"
)

type OutboxWorker struct {
	repo      repository.BillingRepository
	publisher broker.EventPublisher
	ctx       context.Context
}

func NewOutboxWorker(ctx context.Context, repo repository.BillingRepository, publisher broker.EventPublisher) *OutboxWorker {
	return &OutboxWorker{
		repo:      repo,
		publisher: publisher,
		ctx:       ctx,
	}
}

func (w *OutboxWorker) StartOutboxWorker() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processOutboxBatch()
		case <-w.ctx.Done():
			fmt.Println("billing outbox worker stopped gracefully")
			return
		}
	}
}

func (w *OutboxWorker) processOutboxBatch() {
	tx, err := w.repo.BeginTx(w.ctx)
	if err != nil {
		fmt.Printf("outbox worker: %v\n", err)
		return
	}
	defer func() {
		err = tx.Rollback(w.ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			fmt.Printf("outbox worker transaction rollback failed: %v\n", err)
		}
	}()

	events, err := w.repo.QueryOutboxEventsTx(w.ctx, tx, 100)
	if err != nil {
		fmt.Printf("outbox: %v\n", err)
		return
	}
	if len(events) == 0 {
		return
	}

	var processedIds []int64
	for _, event := range events {
		err := w.publisher.Publish(w.ctx, event)
		if err != nil {
			fmt.Printf("outbox worker: %v\n", err)
			continue
		}
		processedIds = append(processedIds, event.Id)
	}

	if len(processedIds) > 0 {
		updated, err := w.repo.MarkEventsAsProcessedTx(w.ctx, tx, processedIds)
		if err != nil {
			fmt.Printf("outbox worker: %v\n", err)
			return
		}

		fmt.Printf("outbox processed %d outbox events\n", updated)
	}

	if err := tx.Commit(w.ctx); err != nil {
		fmt.Printf("outbox worker: %v\n", err)
	}
}
