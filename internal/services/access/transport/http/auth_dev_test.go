package httptransport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestDevelopmentAuthResolverReturnsDefaultReadScope(t *testing.T) {
	resolver := NewDevelopmentAuthResolver(DevelopmentAuthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	req.Header.Set("X-Request-Id", "req-auth-1")

	auth, err := resolver(req)
	if err != nil {
		t.Fatalf("resolver error = %v", err)
	}
	if auth.TenantID != "demo" || auth.SubjectID != "dev-user" {
		t.Fatalf("unexpected auth context: %#v", auth)
	}
	if !auth.HasScope(domain.ScopeFileRead) {
		t.Fatalf("expected %q scope, got %#v", domain.ScopeFileRead, auth.Scopes)
	}
	if auth.RequestID != "req-auth-1" {
		t.Fatalf("RequestID = %q, want %q", auth.RequestID, "req-auth-1")
	}
}

func TestDevelopmentAuthResolverRejectsUnexpectedToken(t *testing.T) {
	resolver := NewDevelopmentAuthResolver(DevelopmentAuthConfig{Token: "expected-token"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	_, err := resolver(req)
	if code := xerrors.CodeOf(err); code != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeUnauthorized, err)
	}
}
