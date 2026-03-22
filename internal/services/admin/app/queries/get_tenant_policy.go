package queries

import (
	"context"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type GetTenantPolicyQuery struct {
	TenantID string
	Auth     domain.AdminContext
}

type GetTenantPolicyHandler struct {
	policies ports.TenantPolicyRepository
}

func NewGetTenantPolicyHandler(policies ports.TenantPolicyRepository) GetTenantPolicyHandler {
	return GetTenantPolicyHandler{policies: policies}
}

func (h GetTenantPolicyHandler) Handle(ctx context.Context, query GetTenantPolicyQuery) (view.TenantPolicy, error) {
	tenantID := strings.TrimSpace(query.TenantID)
	if tenantID == "" {
		return view.TenantPolicy{}, xerrors.New(xerrors.CodeInvalidArgument, "tenant id is required", xerrors.Details{
			"field": "tenantId",
		})
	}
	if err := authorizeTenantOperation(query.Auth, domain.OperationGetTenantPolicy, tenantID); err != nil {
		return view.TenantPolicy{}, err
	}

	policy, err := h.policies.GetByTenantID(ctx, tenantID)
	if err != nil {
		return view.TenantPolicy{}, err
	}
	if policy == nil {
		return view.TenantPolicy{}, domain.ErrTenantNotFound(tenantID)
	}

	return view.FromTenantPolicy(policy), nil
}
