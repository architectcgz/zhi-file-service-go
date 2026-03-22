package ports

import (
	"context"
	"time"
)

type TenantUsageView struct {
	TenantID      string
	StorageBytes  int64
	FileCount     int64
	UpdatedAt     time.Time
}

type TenantUsageQuery interface {
	GetByTenantID(ctx context.Context, tenantID string) (*TenantUsageView, error)
}
