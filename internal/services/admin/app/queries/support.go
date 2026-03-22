package queries

import (
	"slices"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
)

func authorizeOperation(ctx domain.AdminContext, operation domain.Operation) error {
	return domain.AuthorizeOperation(ctx, operation)
}

func authorizeTenantOperation(ctx domain.AdminContext, operation domain.Operation, tenantID string) error {
	if err := domain.AuthorizeOperation(ctx, operation); err != nil {
		return err
	}

	return domain.EnsureTenantScope(ctx, tenantID)
}

func scopedTenants(ctx domain.AdminContext) []string {
	if ctx.IsGlobalScope() {
		return nil
	}

	return slices.Clone(ctx.TenantScopes)
}
