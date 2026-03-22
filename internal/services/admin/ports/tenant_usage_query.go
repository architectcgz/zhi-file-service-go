package ports

import (
	"context"
	"time"
)

type TenantUsageView struct {
	TenantID     string
	StorageBytes int64
	FileCount    int64
	LastUploadAt *time.Time
	UpdatedAt    time.Time
}

type TenantUsageRepository interface {
	Initialize(ctx context.Context, tenantID string) error
	GetByTenantID(ctx context.Context, tenantID string) (*TenantUsageView, error)
}
