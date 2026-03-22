package httptransport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestDevelopmentAuthResolverUsesDefaults(t *testing.T) {
	t.Parallel()

	resolver := NewDevelopmentAuthResolver(DevelopmentAuthConfig{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload-sessions/upload-1", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("X-Request-Id", "req-auth-1")

	auth, err := resolver(req)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if auth.RequestID != "req-auth-1" {
		t.Fatalf("RequestID = %q, want %q", auth.RequestID, "req-auth-1")
	}
	if auth.TenantID != "demo" || auth.SubjectID != "dev-user" || auth.SubjectType != "USER" {
		t.Fatalf("unexpected auth context: %#v", auth)
	}
	if len(auth.Scopes) != 1 || auth.Scopes[0] != domain.ScopeFileWrite {
		t.Fatalf("Scopes = %#v, want [%q]", auth.Scopes, domain.ScopeFileWrite)
	}
}

func TestDevelopmentAuthResolverUsesCustomConfig(t *testing.T) {
	t.Parallel()

	resolver := NewDevelopmentAuthResolver(DevelopmentAuthConfig{
		Token:       "custom-token",
		TenantID:    "tenant-a",
		SubjectID:   "user-1",
		SubjectType: "APP",
		ClientID:    "cli",
		TokenID:     "token-1",
		Scopes:      []string{"file:read", "file:write"},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload-sessions/upload-1", nil)
	req.Header.Set("Authorization", "Bearer custom-token")

	auth, err := resolver(req)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if auth.TenantID != "tenant-a" || auth.SubjectID != "user-1" || auth.SubjectType != "APP" {
		t.Fatalf("unexpected auth context: %#v", auth)
	}
	if auth.ClientID != "cli" || auth.TokenID != "token-1" {
		t.Fatalf("unexpected client/token context: %#v", auth)
	}
	if len(auth.Scopes) != 2 {
		t.Fatalf("Scopes = %#v, want 2 scopes", auth.Scopes)
	}
}

func TestDevelopmentAuthResolverRejectsUnexpectedToken(t *testing.T) {
	t.Parallel()

	resolver := NewDevelopmentAuthResolver(DevelopmentAuthConfig{Token: "custom-token"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload-sessions/upload-1", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	_, err := resolver(req)
	if xerrors.CodeOf(err) != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", xerrors.CodeOf(err), xerrors.CodeUnauthorized, err)
	}
}
