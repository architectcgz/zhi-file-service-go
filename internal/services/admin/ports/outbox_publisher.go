package ports

import (
	"context"
	"time"
)

type OutboxEvent struct {
	EventType     string
	AggregateType string
	AggregateID   string
	OccurredAt    time.Time
	RequestID     string
	TenantID      string
	Payload       map[string]any
}

type OutboxPublisher interface {
	Publish(ctx context.Context, event OutboxEvent) error
}
