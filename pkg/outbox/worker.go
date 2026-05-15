package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Repository interface {
	QueryOutboxEventsTx(ctx context.Context, tx pgx.Tx, limit int) ([]*Event, error)
	MarkEventsAsProcessedTx(ctx context.Context, tx pgx.Tx, eventIds []int64) (int64, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
	InsertOutboxEventTx(ctx context.Context, tx pgx.Tx, eventType, aggregateId string, payload any) error
}

type Event struct {
	Id            int64
	AggregateType string
	AggregateId   string
	EventType     string
	Payload       map[string]any
	Status        string
	ErrorText     *string
	CreatedAt     *time.Time
	ProcessedAt   *time.Time
}

type EventPublisher interface {
	Publish(ctx context.Context, event *Event) error
}

type Worker struct {
	repo      Repository
	publisher EventPublisher
}

func NewOutboxWorker(repo Repository, publisher EventPublisher) *Worker {
	return &Worker{
		repo:      repo,
		publisher: publisher,
	}
}

func (w *Worker) StartOutboxWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processOutboxBatch(ctx)
		case <-ctx.Done():
			fmt.Println("billing outbox worker stopped gracefully")
			return
		}
	}
}

func (w *Worker) processOutboxBatch(ctx context.Context) {
	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		fmt.Printf("outbox worker: %v\n", err)
		return
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			fmt.Printf("outbox worker transaction rollback failed: %v\n", err)
		}
	}()

	events, err := w.repo.QueryOutboxEventsTx(ctx, tx, 100)
	if err != nil {
		fmt.Printf("outbox: %v\n", err)
		return
	}
	if len(events) == 0 {
		return
	}

	var processedIds []int64
	for _, event := range events {
		err := w.publisher.Publish(ctx, event)
		if err != nil {
			fmt.Printf("outbox worker: %v\n", err)
			continue
		}
		processedIds = append(processedIds, event.Id)
	}

	if len(processedIds) > 0 {
		updated, err := w.repo.MarkEventsAsProcessedTx(ctx, tx, processedIds)
		if err != nil {
			fmt.Printf("outbox worker: %v\n", err)
			return
		}

		fmt.Printf("outbox processed %d outbox events\n", updated)
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Printf("outbox worker: %v\n", err)
	}
}
