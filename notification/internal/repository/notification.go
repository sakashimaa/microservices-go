package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/billing-microservice/notification/internal/domain"
)

var (
	ErrDuplicateMessage = errors.New("message already processed")
)

type InboxRepository interface {
	SaveProcessedMessageTx(ctx context.Context, tx pgx.Tx, params domain.SaveProcessedMessageParams) error
}

type inboxPGRepo struct {
	db *pgxpool.Pool
}

func NewInboxRepository(db *pgxpool.Pool) InboxRepository {
	return &inboxPGRepo{
		db: db,
	}
}

func (i *inboxPGRepo) SaveProcessedMessageTx(ctx context.Context, tx pgx.Tx, params domain.SaveProcessedMessageParams) error {
	query := `
		INSERT INTO processed_events (
			event_id, aggregate_id, aggregate_type, event_type, processed_at
		) VALUES ($1, $2, $3, $4, NOW())
	`

	if _, err := tx.Exec(
		ctx,
		query,
		params.EventId,
		params.AggregateId,
		params.AggregateType,
		params.EventType,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateMessage
		}
		return err
	}

	return nil
}
