-- +goose Up
CREATE TABLE IF NOT EXISTS outbox_events (
     id BIGSERIAL PRIMARY KEY,
     aggregate_type VARCHAR(255) NOT NULL,
     aggregate_id UUID NOT NULL,
     event_type VARCHAR(255) NOT NULL,
     payload JSONB NOT NULL,
     status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
     error_text TEXT,
     created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
     processed_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_outbox_status_id ON outbox_events(status);

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_status_id;
DROP TABLE IF EXISTS outbox_events;
