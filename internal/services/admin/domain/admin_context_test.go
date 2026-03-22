package domain

import "testing"

func TestNewAdminContext_NormalizesClaims(t *testing.T) {
	t.Parallel()

	ctx, err := NewAdminContext(AdminContextInput{
		RequestID:    "req-1",
		AdminID:      "admin-1",
		Roles:        []string{" admin.readonly ", "admin.governance", "admin.readonly", ""},
		TenantScopes: []string{"tenant-a", " * ", "tenant-b"},
		Permissions:  []string{"tenant.read", "tenant.read", " tenant.write "},
		TokenID:      "token-1",
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}

	if ctx.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want %q", ctx.RequestID, "req-1")
	}
	if ctx.AdminID != "admin-1" {
		t.Fatalf("AdminID = %q, want %q", ctx.AdminID, "admin-1")
	}
	if len(ctx.Roles) != 2 {
		t.Fatalf("len(Roles) = %d, want %d", len(ctx.Roles), 2)
	}
	if ctx.Roles[0] != RoleReadonly || ctx.Roles[1] != RoleGovernance {
		t.Fatalf("Roles = %#v, want [%q %q]", ctx.Roles, RoleReadonly, RoleGovernance)
	}
	if len(ctx.TenantScopes) != 1 || ctx.TenantScopes[0] != GlobalTenantScope {
		t.Fatalf("TenantScopes = %#v, want [%q]", ctx.TenantScopes, GlobalTenantScope)
	}
	if len(ctx.Permissions) != 2 || ctx.Permissions[0] != "tenant.read" || ctx.Permissions[1] != "tenant.write" {
		t.Fatalf("Permissions = %#v, want [tenant.read tenant.write]", ctx.Permissions)
	}
	if !ctx.IsGlobalScope() {
		t.Fatalf("IsGlobalScope() = false, want true")
	}
	if !ctx.CanAccessTenant("tenant-c") {
		t.Fatalf("CanAccessTenant() = false, want true for global scope")
	}
}

func TestNewAdminContext_RejectsMissingAdminID(t *testing.T) {
	t.Parallel()

	_, err := NewAdminContext(AdminContextInput{})
	if err == nil {
		t.Fatal("NewAdminContext() error = nil, want non-nil")
	}
}

func TestAdminContext_HasMinimumRole(t *testing.T) {
	t.Parallel()

	governance, err := NewAdminContext(AdminContextInput{
		AdminID: "admin-1",
		Roles:   []string{string(RoleGovernance)},
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}
	if !governance.HasMinimumRole(RoleReadonly) {
		t.Fatal("HasMinimumRole(readonly) = false, want true")
	}
	if !governance.HasMinimumRole(RoleGovernance) {
		t.Fatal("HasMinimumRole(governance) = false, want true")
	}
	if governance.HasMinimumRole(RoleSuper) {
		t.Fatal("HasMinimumRole(super) = true, want false")
	}

	superAdmin, err := NewAdminContext(AdminContextInput{
		AdminID: "admin-2",
		Roles:   []string{string(RoleSuper)},
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}
	if !superAdmin.HasMinimumRole(RoleGovernance) {
		t.Fatal("super HasMinimumRole(governance) = false, want true")
	}
}
