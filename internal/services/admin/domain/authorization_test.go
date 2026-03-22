package domain

import (
	"testing"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestAuthorizeOperation_UsesRoleMatrix(t *testing.T) {
	t.Parallel()

	ctx, err := NewAdminContext(AdminContextInput{
		AdminID: "readonly-admin",
		Roles:   []string{string(RoleReadonly)},
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}

	if err := AuthorizeOperation(ctx, OperationListTenants); err != nil {
		t.Fatalf("AuthorizeOperation(list) error = %v, want nil", err)
	}

	err = AuthorizeOperation(ctx, OperationDeleteFile)
	if err == nil {
		t.Fatal("AuthorizeOperation(delete) error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != CodeAdminPermissionDenied {
		t.Fatalf("CodeOf() = %q, want %q", code, CodeAdminPermissionDenied)
	}
	if status := xerrors.StatusOf(err); status != 403 {
		t.Fatalf("StatusOf() = %d, want %d", status, 403)
	}
}

func TestEnsureTenantScope_DeniesOutOfScopeTenant(t *testing.T) {
	t.Parallel()

	ctx, err := NewAdminContext(AdminContextInput{
		AdminID:      "governance-admin",
		Roles:        []string{string(RoleGovernance)},
		TenantScopes: []string{"tenant-a"},
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}

	err = EnsureTenantScope(ctx, "tenant-b")
	if err == nil {
		t.Fatal("EnsureTenantScope() error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != CodeTenantScopeDenied {
		t.Fatalf("CodeOf() = %q, want %q", code, CodeTenantScopeDenied)
	}
}

func TestEnsureTenantScope_RejectsBlankTenantID(t *testing.T) {
	t.Parallel()

	ctx, err := NewAdminContext(AdminContextInput{
		AdminID: "readonly-admin",
		Roles:   []string{string(RoleReadonly)},
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}

	err = EnsureTenantScope(ctx, "   ")
	if err == nil {
		t.Fatal("EnsureTenantScope() error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != xerrors.CodeInvalidArgument {
		t.Fatalf("CodeOf() = %q, want %q", code, xerrors.CodeInvalidArgument)
	}
}

func TestRequireDestructiveReason_RejectsBlankReason(t *testing.T) {
	t.Parallel()

	err := RequireDestructiveReason("   ")
	if err == nil {
		t.Fatal("RequireDestructiveReason() error = nil, want non-nil")
	}
	if code := xerrors.CodeOf(err); code != xerrors.CodeInvalidArgument {
		t.Fatalf("CodeOf() = %q, want %q", code, xerrors.CodeInvalidArgument)
	}
}

func TestOperationRule_MapsOpenAPIContract(t *testing.T) {
	t.Parallel()

	rule, ok := RuleFor(OperationCreateTenant)
	if !ok {
		t.Fatal("RuleFor(create tenant) ok = false, want true")
	}
	if rule.MinimumRole != RoleSuper {
		t.Fatalf("MinimumRole = %q, want %q", rule.MinimumRole, RoleSuper)
	}

	deleteRule, ok := RuleFor(OperationDeleteFile)
	if !ok {
		t.Fatal("RuleFor(delete file) ok = false, want true")
	}
	if !deleteRule.Destructive {
		t.Fatal("DeleteFile rule Destructive = false, want true")
	}
}
