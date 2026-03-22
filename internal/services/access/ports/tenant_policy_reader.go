package ports

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
)

type TenantPolicyReader interface {
	GetByTenantID(ctx context.Context, tenantID string) (domain.TenantPolicy, error)
}
