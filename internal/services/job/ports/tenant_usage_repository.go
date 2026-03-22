package ports

import "context"

type TenantUsageRepository interface {
	ReconcileTenantUsage(ctx context.Context, limit int) (int, error)
}
