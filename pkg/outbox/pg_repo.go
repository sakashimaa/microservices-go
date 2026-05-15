package outbox

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgxRepository struct {
	pool *pgxpool.Pool
}

func NewPgxRepository(pool *pgxpool.Pool) *PgxRepository {
	return &PgxRepository{pool: pool}
}

func (r *PgxRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("transaction start failed: %w", err)
	}

	return tx, nil
}

func (r *PgxRepository) QueryOutboxEventsTx(ctx context.Context, tx pgx.Tx, limit int) ([]*Event, error) {
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

	var res []*Event
	for rows.Next() {
		var e Event
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

func (r *PgxRepository) InsertOutboxEventTx(ctx context.Context, tx pgx.Tx, eventType, aggregateId string, payload any) error {
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

func (r *PgxRepository) MarkEventsAsProcessedTx(ctx context.Context, tx pgx.Tx, eventIds []int64) (int64, error) {
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
