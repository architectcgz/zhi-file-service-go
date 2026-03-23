package httptransport

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestJWKSAuthResolverAcceptsInlineJWKS(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "admin-key-inline")
	resolver, err := NewJWKSAuthResolver(key.jwksJSON(t))
	if err != nil {
		t.Fatalf("NewJWKSAuthResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/tenants", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":           "admin-001",
		"roles":         []string{string(domain.RoleReadonly), string(domain.RoleGovernance)},
		"iss":           "https://sso.example.com",
		"aud":           []string{"zhi-file-admin", "other-audience"},
		"iat":           time.Now().Add(-time.Minute).Unix(),
		"exp":           time.Now().Add(time.Hour).Unix(),
		"tenant_scopes": []string{"tenant-a", "tenant-a", "tenant-b"},
		"permissions":   []string{"tenant.read", "tenant.write", "tenant.write"},
		"jti":           "token-001",
	}))
	req.Header.Set("X-Request-Id", "req-admin-jwks-1")

	auth, err := resolver(req)
	if err != nil {
		t.Fatalf("resolver() error = %v", err)
	}
	if auth.RequestID != "req-admin-jwks-1" {
		t.Fatalf("RequestID = %q, want %q", auth.RequestID, "req-admin-jwks-1")
	}
	if auth.AdminID != "admin-001" {
		t.Fatalf("AdminID = %q, want %q", auth.AdminID, "admin-001")
	}
	if !auth.HasMinimumRole(domain.RoleGovernance) {
		t.Fatalf("expected %q in auth roles, got %#v", domain.RoleGovernance, auth.Roles)
	}
	if auth.HasMinimumRole(domain.RoleSuper) {
		t.Fatalf("expected auth roles %#v not to include %q", auth.Roles, domain.RoleSuper)
	}
	if len(auth.TenantScopes) != 2 || auth.TenantScopes[0] != "tenant-a" || auth.TenantScopes[1] != "tenant-b" {
		t.Fatalf("TenantScopes = %#v, want [tenant-a tenant-b]", auth.TenantScopes)
	}
	if len(auth.Permissions) != 2 || auth.Permissions[0] != "tenant.read" || auth.Permissions[1] != "tenant.write" {
		t.Fatalf("Permissions = %#v, want [tenant.read tenant.write]", auth.Permissions)
	}
	if auth.TokenID != "token-001" {
		t.Fatalf("TokenID = %q, want %q", auth.TokenID, "token-001")
	}
}

func TestJWKSAuthResolverRejectsAudienceMismatch(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "admin-key-aud")
	resolver, err := NewJWKSAuthResolver(key.jwksJSON(t))
	if err != nil {
		t.Fatalf("NewJWKSAuthResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/tenants", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":   "admin-002",
		"roles": []string{string(domain.RoleReadonly)},
		"iss":   "https://sso.example.com",
		"aud":   "zhi-file-upload",
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	}))

	_, err = resolver(req)
	if code := xerrors.CodeOf(err); code != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeUnauthorized, err)
	}
}

func TestJWKSAuthResolverRejectsIssuerOutsideAllowlist(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "admin-key-iss")
	resolver, err := NewJWKSAuthResolverWithIssuers(key.jwksJSON(t), []string{"https://trusted-issuer.example.com"})
	if err != nil {
		t.Fatalf("NewJWKSAuthResolverWithIssuers() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/tenants", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":   "admin-issuer",
		"roles": []string{string(domain.RoleReadonly)},
		"iss":   "https://unexpected-issuer.example.com",
		"aud":   "zhi-file-admin",
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	}))

	_, err = resolver(req)
	if code := xerrors.CodeOf(err); code != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeUnauthorized, err)
	}
}

func TestJWKSAuthResolverRejectsUnknownKeyID(t *testing.T) {
	t.Parallel()

	jwksKey := newTestJWKSRSAKey(t, "admin-key-known")
	tokenKey := newTestJWKSRSAKey(t, "admin-key-missing")
	resolver, err := NewJWKSAuthResolver(jwksKey.jwksJSON(t))
	if err != nil {
		t.Fatalf("NewJWKSAuthResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/tenants", nil)
	req.Header.Set("Authorization", "Bearer "+tokenKey.signedToken(t, map[string]any{
		"sub":   "admin-003",
		"roles": []string{string(domain.RoleReadonly)},
		"iss":   "https://sso.example.com",
		"aud":   "zhi-file-admin",
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	}))

	_, err = resolver(req)
	if code := xerrors.CodeOf(err); code != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeUnauthorized, err)
	}
}

func TestJWKSAuthResolverLoadsJWKSFromURL(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "admin-key-url")
	jwks := key.jwksJSON(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, jwks)
	}))
	defer server.Close()

	resolver, err := NewJWKSAuthResolver(server.URL)
	if err != nil {
		t.Fatalf("NewJWKSAuthResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/files", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":   "admin-004",
		"roles": []string{string(domain.RoleSuper)},
		"iss":   "https://sso.example.com",
		"aud":   []string{"zhi-file-admin"},
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	}))

	auth, err := resolver(req)
	if err != nil {
		t.Fatalf("resolver() error = %v", err)
	}
	if auth.AdminID != "admin-004" {
		t.Fatalf("AdminID = %q, want %q", auth.AdminID, "admin-004")
	}
	if !auth.HasMinimumRole(domain.RoleSuper) {
		t.Fatalf("expected %q in auth roles, got %#v", domain.RoleSuper, auth.Roles)
	}
}

func TestJWKSAuthResolverRefreshesRemoteJWKSWhenSignatureUsesRotatedKey(t *testing.T) {
	t.Parallel()

	oldKey := newTestJWKSRSAKey(t, "admin-key-rotated")
	newKey := newTestJWKSRSAKey(t, "admin-key-rotated")

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if requests.Add(1) == 1 {
			_, _ = io.WriteString(w, oldKey.jwksJSON(t))
			return
		}
		_, _ = io.WriteString(w, newKey.jwksJSON(t))
	}))
	defer server.Close()

	resolver, err := NewJWKSAuthResolver(server.URL)
	if err != nil {
		t.Fatalf("NewJWKSAuthResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/files", nil)
	req.Header.Set("Authorization", "Bearer "+newKey.signedToken(t, map[string]any{
		"sub":   "admin-005",
		"roles": []string{string(domain.RoleSuper)},
		"iss":   "https://sso.example.com",
		"aud":   []string{"zhi-file-admin"},
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	}))

	auth, err := resolver(req)
	if err != nil {
		t.Fatalf("resolver() error = %v", err)
	}
	if auth.AdminID != "admin-005" {
		t.Fatalf("AdminID = %q, want %q", auth.AdminID, "admin-005")
	}
	if requests.Load() < 2 {
		t.Fatalf("jwks requests = %d, want at least 2", requests.Load())
	}
}

type testJWKSRSAKey struct {
	privateKey *rsa.PrivateKey
	kid        string
}

func newTestJWKSRSAKey(t *testing.T, kid string) testJWKSRSAKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	return testJWKSRSAKey{
		privateKey: privateKey,
		kid:        kid,
	}
}

func (k testJWKSRSAKey) jwksJSON(t *testing.T) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": k.kid,
				"use": "sig",
				"alg": "RS256",
				"n":   base64RawURLEncode(k.privateKey.PublicKey.N.Bytes()),
				"e":   base64RawURLEncode(big.NewInt(int64(k.privateKey.PublicKey.E)).Bytes()),
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(payload)
}

func (k testJWKSRSAKey) signedToken(t *testing.T, claims map[string]any) string {
	t.Helper()

	headerJSON, err := json.Marshal(map[string]any{
		"alg": "RS256",
		"kid": k.kid,
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("json.Marshal(header) error = %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("json.Marshal(claims) error = %v", err)
	}

	signingInput := base64RawURLEncode(headerJSON) + "." + base64RawURLEncode(claimsJSON)
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, k.privateKey, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("rsa.SignPKCS1v15() error = %v", err)
	}

	return signingInput + "." + base64RawURLEncode(signature)
}

func base64RawURLEncode(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
