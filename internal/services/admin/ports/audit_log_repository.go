package ports

import (
	"context"
	"time"
)

type AuditLogRecord struct {
	AuditID      string
	AdminSubject string
	TenantID     string
	Action       string
	RequestID    string
	Details      map[string]any
	CreatedAt    time.Time
}

type ListAuditLogsQuery struct {
	TenantID string
	Cursor   string
	Limit    int
}

type AuditLogRepository interface {
	Append(ctx context.Context, record AuditLogRecord) error
	List(ctx context.Context, query ListAuditLogsQuery) ([]AuditLogRecord, string, error)
}
