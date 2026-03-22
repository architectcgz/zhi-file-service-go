package app

import (
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestGuard_EnsureTenantPatchRequiresReasonForDestructiveStatus(t *testing.T) {
	t.Parallel()

	guard := NewGuard()
	ctx := mustAdminContext(t, domain.AdminContextInput{
		AdminID:      "governance-admin",
		Roles:        []string{string(domain.RoleGovernance)},
		TenantScopes: []string{"tenant-a"},
	})

	err := guard.EnsureTenantPatch(ctx, "tenant-a", domain.TenantPatch{
		Status: domain.TenantStatusPtr(domain.TenantStatusSuspended),
	})
	if err == nil {
		t.Fatal("EnsureTenantPatch() error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != xerrors.CodeInvalidArgument {
		t.Fatalf("CodeOf() = %q, want %q", code, xerrors.CodeInvalidArgument)
	}
}

func TestGuard_EnsureCreateTenantRequiresSuperRole(t *testing.T) {
	t.Parallel()

	guard := NewGuard()
	governance := mustAdminContext(t, domain.AdminContextInput{
		AdminID:      "governance-admin",
		Roles:        []string{string(domain.RoleGovernance)},
		TenantScopes: []string{"tenant-a"},
	})

	err := guard.EnsureCreateTenant(governance, "tenant-a")
	if err == nil {
		t.Fatal("EnsureCreateTenant() error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != domain.CodeAdminPermissionDenied {
		t.Fatalf("CodeOf() = %q, want %q", code, domain.CodeAdminPermissionDenied)
	}

	superAdmin := mustAdminContext(t, domain.AdminContextInput{
		AdminID:      "super-admin",
		Roles:        []string{string(domain.RoleSuper)},
		TenantScopes: []string{"tenant-a"},
	})
	if err := guard.EnsureCreateTenant(superAdmin, "tenant-a"); err != nil {
		t.Fatalf("EnsureCreateTenant() error = %v, want nil", err)
	}
}

func TestGuard_EnsureTenantPatchChecksTenantScope(t *testing.T) {
	t.Parallel()

	guard := NewGuard()
	ctx := mustAdminContext(t, domain.AdminContextInput{
		AdminID:      "governance-admin",
		Roles:        []string{string(domain.RoleGovernance)},
		TenantScopes: []string{"tenant-a"},
	})

	err := guard.EnsureTenantPatch(ctx, "tenant-b", domain.TenantPatch{
		Status: domain.TenantStatusPtr(domain.TenantStatusActive),
	})
	if err == nil {
		t.Fatal("EnsureTenantPatch() error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != domain.CodeTenantScopeDenied {
		t.Fatalf("CodeOf() = %q, want %q", code, domain.CodeTenantScopeDenied)
	}
}

func TestGuard_EnsureTenantPolicyPatchRequiresReasonWhenTightening(t *testing.T) {
	t.Parallel()

	guard := NewGuard()
	ctx := mustAdminContext(t, domain.AdminContextInput{
		AdminID:      "governance-admin",
		Roles:        []string{string(domain.RoleGovernance)},
		TenantScopes: []string{"tenant-a"},
	})

	current := domain.TenantPolicy{
		MaxStorageBytes: int64Ptr(100),
	}
	next := domain.TenantPolicyPatch{
		MaxStorageBytes: int64Ptr(80),
	}

	err := guard.EnsureTenantPolicyPatch(ctx, "tenant-a", current, next)
	if err == nil {
		t.Fatal("EnsureTenantPolicyPatch() error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != xerrors.CodeInvalidArgument {
		t.Fatalf("CodeOf() = %q, want %q", code, xerrors.CodeInvalidArgument)
	}
}

func TestGuard_EnsureDeleteFileRequiresGovernanceAndReason(t *testing.T) {
	t.Parallel()

	guard := NewGuard()
	readonly := mustAdminContext(t, domain.AdminContextInput{
		AdminID:      "readonly-admin",
		Roles:        []string{string(domain.RoleReadonly)},
		TenantScopes: []string{"tenant-a"},
	})

	err := guard.EnsureDeleteFile(readonly, "tenant-a", "")
	if err == nil {
		t.Fatal("EnsureDeleteFile() error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != domain.CodeAdminPermissionDenied {
		t.Fatalf("CodeOf() = %q, want %q", code, domain.CodeAdminPermissionDenied)
	}

	governance := mustAdminContext(t, domain.AdminContextInput{
		AdminID:      "governance-admin",
		Roles:        []string{string(domain.RoleGovernance)},
		TenantScopes: []string{"tenant-a"},
	})

	err = guard.EnsureDeleteFile(governance, "tenant-a", "")
	if err == nil {
		t.Fatal("EnsureDeleteFile() blank reason error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != xerrors.CodeInvalidArgument {
		t.Fatalf("blank reason CodeOf() = %q, want %q", code, xerrors.CodeInvalidArgument)
	}

	if err := guard.EnsureDeleteFile(governance, "tenant-a", "manual cleanup"); err != nil {
		t.Fatalf("EnsureDeleteFile() error = %v, want nil", err)
	}
}

func mustAdminContext(t *testing.T, input domain.AdminContextInput) domain.AdminContext {
	t.Helper()

	ctx, err := domain.NewAdminContext(input)
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}

	return ctx
}

func int64Ptr(value int64) *int64 {
	return &value
}
