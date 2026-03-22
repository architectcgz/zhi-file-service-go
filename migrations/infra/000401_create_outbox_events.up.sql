CREATE TABLE infra.outbox_events (
  event_id VARCHAR(26) PRIMARY KEY,
  service_name VARCHAR(64) NOT NULL,
  aggregate_type VARCHAR(64) NOT NULL,
  aggregate_id VARCHAR(64) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  payload JSONB NOT NULL,
  status VARCHAR(16) NOT NULL,
  retry_count INTEGER NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT ck_outbox_events_status CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),
  CONSTRAINT ck_outbox_events_retry_non_negative CHECK (retry_count >= 0)
);

CREATE INDEX idx_outbox_events_status_next
  ON infra.outbox_events (status, next_attempt_at, created_at);

CREATE INDEX idx_outbox_events_aggregate
  ON infra.outbox_events (aggregate_type, aggregate_id);
