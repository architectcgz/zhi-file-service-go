package ports

import "context"

type OutboxMessage struct {
	EventType   string
	AggregateID string
	Payload     []byte
}

type OutboxPublisher interface {
	Enqueue(ctx context.Context, message OutboxMessage) error
}
