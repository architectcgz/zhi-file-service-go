package httptransport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestDevelopmentAuthResolverUsesDefaults(t *testing.T) {
	t.Parallel()

	resolver := NewDevelopmentAuthResolver(DevelopmentAuthConfig{})
	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/tenants", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("X-Request-Id", "req-admin-auth-1")

	auth, err := resolver(req)
	if err != nil {
		t.Fatalf("resolver error = %v", err)
	}
	if auth.RequestID != "req-admin-auth-1" {
		t.Fatalf("RequestID = %q, want %q", auth.RequestID, "req-admin-auth-1")
	}
	if auth.AdminID != "admin-dev" {
		t.Fatalf("AdminID = %q, want %q", auth.AdminID, "admin-dev")
	}
	if !auth.HasMinimumRole(domain.RoleSuper) {
		t.Fatalf("expected %q in auth roles, got %#v", domain.RoleSuper, auth.Roles)
	}
	if !auth.IsGlobalScope() {
		t.Fatalf("expected global scope, got %#v", auth.TenantScopes)
	}
}

func TestDevelopmentAuthResolverRejectsUnexpectedToken(t *testing.T) {
	t.Parallel()

	resolver := NewDevelopmentAuthResolver(DevelopmentAuthConfig{Token: "expected-token"})
	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/tenants", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	_, err := resolver(req)
	if code := xerrors.CodeOf(err); code != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeUnauthorized, err)
	}
}
