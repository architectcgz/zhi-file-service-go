package ports

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
)

type TenantPolicyRepository interface {
	GetByTenantID(ctx context.Context, tenantID string) (*domain.TenantPolicy, error)
	Patch(ctx context.Context, tenantID string, patch domain.TenantPolicyPatch) (*domain.TenantPolicy, error)
}
