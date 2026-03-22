package queries

import (
	"context"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type GetTenantQuery struct {
	TenantID string
	Auth     domain.AdminContext
}

type GetTenantHandler struct {
	tenants ports.TenantRepository
}

func NewGetTenantHandler(tenants ports.TenantRepository) GetTenantHandler {
	return GetTenantHandler{tenants: tenants}
}

func (h GetTenantHandler) Handle(ctx context.Context, query GetTenantQuery) (view.Tenant, error) {
	tenantID := strings.TrimSpace(query.TenantID)
	if tenantID == "" {
		return view.Tenant{}, xerrors.New(xerrors.CodeInvalidArgument, "tenant id is required", xerrors.Details{
			"field": "tenantId",
		})
	}
	if err := authorizeTenantOperation(query.Auth, domain.OperationGetTenant, tenantID); err != nil {
		return view.Tenant{}, err
	}

	tenant, err := h.tenants.GetByID(ctx, tenantID)
	if err != nil {
		return view.Tenant{}, err
	}
	if tenant == nil {
		return view.Tenant{}, domain.ErrTenantNotFound(tenantID)
	}

	return view.FromTenant(*tenant), nil
}
