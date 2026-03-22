package ports

import (
	"context"
	"time"
)

type AccessTicketIssueRecord struct {
	Fingerprint string
	Ticket      string
	RedirectURL string
	ExpiresAt   time.Time
}

type AccessTicketIdempotencyStore interface {
	Get(ctx context.Context, key string) (AccessTicketIssueRecord, bool, error)
	PutIfAbsent(ctx context.Context, key string, record AccessTicketIssueRecord, ttl time.Duration) (bool, error)
}
