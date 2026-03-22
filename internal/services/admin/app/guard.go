package app

import "github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"

type Guard struct{}

func NewGuard() Guard {
	return Guard{}
}

func (Guard) EnsureCreateTenant(ctx domain.AdminContext, tenantID string) error {
	if err := domain.AuthorizeOperation(ctx, domain.OperationCreateTenant); err != nil {
		return err
	}

	return domain.EnsureTenantScope(ctx, tenantID)
}

func (Guard) EnsureTenantPatch(ctx domain.AdminContext, tenantID string, patch domain.TenantPatch) error {
	if err := domain.AuthorizeOperation(ctx, domain.OperationPatchTenant); err != nil {
		return err
	}
	if err := domain.EnsureTenantScope(ctx, tenantID); err != nil {
		return err
	}

	patch = patch.Normalize()
	if patch.HasDestructiveChange() {
		return domain.RequireDestructiveReason(patch.Reason)
	}

	return nil
}

func (Guard) EnsureTenantPolicyPatch(ctx domain.AdminContext, tenantID string, current domain.TenantPolicy, patch domain.TenantPolicyPatch) error {
	if err := domain.AuthorizeOperation(ctx, domain.OperationPatchTenantPolicy); err != nil {
		return err
	}
	if err := domain.EnsureTenantScope(ctx, tenantID); err != nil {
		return err
	}

	if patch.ApplyTo(current).TightensComparedTo(current) {
		return domain.RequireDestructiveReason(patch.Reason)
	}

	return nil
}

func (Guard) EnsureDeleteFile(ctx domain.AdminContext, tenantID string, reason string) error {
	if err := domain.AuthorizeOperation(ctx, domain.OperationDeleteFile); err != nil {
		return err
	}
	if err := domain.EnsureTenantScope(ctx, tenantID); err != nil {
		return err
	}

	return domain.RequireDestructiveReason(reason)
}
