package ports

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
)

type ListTenantsQuery struct {
	Cursor       string
	Limit        int
	Status       *domain.TenantStatus
	TenantScopes []string
}

type TenantRepository interface {
	Create(ctx context.Context, tenant domain.Tenant) error
	GetByID(ctx context.Context, tenantID string) (*domain.Tenant, error)
	List(ctx context.Context, query ListTenantsQuery) ([]domain.Tenant, string, error)
	Patch(ctx context.Context, tenantID string, patch domain.TenantPatch) (*domain.Tenant, error)
}
