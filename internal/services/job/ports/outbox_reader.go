package ports

import (
	"context"
	"time"
)

type OutboxEvent struct {
	EventID       string
	ServiceName   string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       map[string]any
	RetryCount    int
	NextAttemptAt *time.Time
}

type ClaimOutboxEventsQuery struct {
	EventTypes []string
	DueBefore  time.Time
	Limit      int
}

type OutboxReader interface {
	ClaimPending(ctx context.Context, query ClaimOutboxEventsQuery) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, eventID string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, eventID string, nextAttemptAt time.Time, lastError string) error
}
