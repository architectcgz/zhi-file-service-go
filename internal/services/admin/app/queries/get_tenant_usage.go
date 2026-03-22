package queries

import (
	"context"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type GetTenantUsageQuery struct {
	TenantID string
	Auth     domain.AdminContext
}

type GetTenantUsageHandler struct {
	usages ports.TenantUsageRepository
}

func NewGetTenantUsageHandler(usages ports.TenantUsageRepository) GetTenantUsageHandler {
	return GetTenantUsageHandler{usages: usages}
}

func (h GetTenantUsageHandler) Handle(ctx context.Context, query GetTenantUsageQuery) (view.TenantUsage, error) {
	tenantID := strings.TrimSpace(query.TenantID)
	if tenantID == "" {
		return view.TenantUsage{}, xerrors.New(xerrors.CodeInvalidArgument, "tenant id is required", xerrors.Details{
			"field": "tenantId",
		})
	}
	if err := authorizeTenantOperation(query.Auth, domain.OperationGetTenantUsage, tenantID); err != nil {
		return view.TenantUsage{}, err
	}

	usage, err := h.usages.GetByTenantID(ctx, tenantID)
	if err != nil {
		return view.TenantUsage{}, err
	}
	if usage == nil {
		return view.TenantUsage{}, domain.ErrTenantNotFound(tenantID)
	}

	return view.FromTenantUsage(usage), nil
}
