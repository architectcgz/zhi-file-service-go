package ports

import (
	"context"
	"time"
)

type SessionPartRecord struct {
	UploadSessionID string
	PartNumber      int
	ETag            string
	PartSize        int64
	Checksum        string
	UploadedAt      time.Time
}

type SessionPartRepository interface {
	ListBySessionID(ctx context.Context, tenantID string, uploadSessionID string) ([]SessionPartRecord, error)
	Upsert(ctx context.Context, record SessionPartRecord) error
	Replace(ctx context.Context, tenantID string, uploadSessionID string, parts []SessionPartRecord) error
}
