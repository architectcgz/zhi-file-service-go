package ports

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
)

type TenantPolicyView struct {
	TenantID  string
	Policy    domain.TenantPolicy
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TenantPolicyRepository interface {
	CreateDefault(ctx context.Context, tenantID string) error
	GetByTenantID(ctx context.Context, tenantID string) (*TenantPolicyView, error)
	Patch(ctx context.Context, tenantID string, patch domain.TenantPolicyPatch) (*TenantPolicyView, error)
}
