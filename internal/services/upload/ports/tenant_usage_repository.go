package ports

import "context"

type TenantUsageRepository interface {
	ApplyDelta(ctx context.Context, tenantID string, deltaBytes int64) error
}
