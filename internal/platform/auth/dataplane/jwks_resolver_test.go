package dataplane

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

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestJWKSResolverAcceptsInlineJWKS(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "data-plane-key-inline")
	resolver, err := NewJWKSResolver(key.jwksJSON(t))
	if err != nil {
		t.Fatalf("NewJWKSResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":          "user-001",
		"subject_type": "USER",
		"tenant_id":    "tenant-a",
		"scope":        []string{"file:read", "file:read", "file:write"},
		"iss":          "https://issuer.example.com",
		"aud":          []string{"zhi-file-data-plane", "other-audience"},
		"iat":          time.Now().Add(-time.Minute).Unix(),
		"exp":          time.Now().Add(time.Hour).Unix(),
		"client_id":    "client-001",
		"jti":          "token-001",
	}))

	claims, err := resolver(req)
	if err != nil {
		t.Fatalf("resolver() error = %v", err)
	}
	if claims.SubjectID != "user-001" {
		t.Fatalf("SubjectID = %q, want %q", claims.SubjectID, "user-001")
	}
	if claims.SubjectType != "USER" {
		t.Fatalf("SubjectType = %q, want %q", claims.SubjectType, "USER")
	}
	if claims.TenantID != "tenant-a" {
		t.Fatalf("TenantID = %q, want %q", claims.TenantID, "tenant-a")
	}
	if claims.ClientID != "client-001" {
		t.Fatalf("ClientID = %q, want %q", claims.ClientID, "client-001")
	}
	if claims.TokenID != "token-001" {
		t.Fatalf("TokenID = %q, want %q", claims.TokenID, "token-001")
	}
	if claims.Issuer != "https://issuer.example.com" {
		t.Fatalf("Issuer = %q, want %q", claims.Issuer, "https://issuer.example.com")
	}
	if len(claims.Scopes) != 2 || claims.Scopes[0] != "file:read" || claims.Scopes[1] != "file:write" {
		t.Fatalf("Scopes = %#v, want [file:read file:write]", claims.Scopes)
	}
}

func TestJWKSResolverRejectsAudienceMismatch(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "data-plane-key-aud")
	resolver, err := NewJWKSResolver(key.jwksJSON(t))
	if err != nil {
		t.Fatalf("NewJWKSResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":          "user-001",
		"subject_type": "USER",
		"tenant_id":    "tenant-a",
		"scope":        "file:read",
		"iss":          "https://issuer.example.com",
		"aud":          "zhi-file-admin",
		"iat":          time.Now().Add(-time.Minute).Unix(),
		"exp":          time.Now().Add(time.Hour).Unix(),
	}))

	_, err = resolver(req)
	if code := xerrors.CodeOf(err); code != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeUnauthorized, err)
	}
}

func TestJWKSResolverRejectsIssuerOutsideAllowlist(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "data-plane-key-iss")
	resolver, err := NewJWKSResolverWithIssuers(key.jwksJSON(t), []string{"https://trusted-issuer.example.com"})
	if err != nil {
		t.Fatalf("NewJWKSResolverWithIssuers() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":          "user-001",
		"subject_type": "USER",
		"tenant_id":    "tenant-a",
		"scope":        "file:read",
		"iss":          "https://unexpected-issuer.example.com",
		"aud":          "zhi-file-data-plane",
		"iat":          time.Now().Add(-time.Minute).Unix(),
		"exp":          time.Now().Add(time.Hour).Unix(),
	}))

	_, err = resolver(req)
	if code := xerrors.CodeOf(err); code != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeUnauthorized, err)
	}
}

func TestJWKSResolverLoadsJWKSFromURL(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "data-plane-key-url")
	jwks := key.jwksJSON(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, jwks)
	}))
	defer server.Close()

	resolver, err := NewJWKSResolver(server.URL)
	if err != nil {
		t.Fatalf("NewJWKSResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":          "user-001",
		"subject_type": "APP",
		"tenant_id":    "tenant-a",
		"scope":        []string{"file:read"},
		"iss":          "https://issuer.example.com",
		"aud":          []string{"zhi-file-data-plane"},
		"iat":          time.Now().Add(-time.Minute).Unix(),
		"exp":          time.Now().Add(time.Hour).Unix(),
	}))

	claims, err := resolver(req)
	if err != nil {
		t.Fatalf("resolver() error = %v", err)
	}
	if claims.SubjectType != "APP" {
		t.Fatalf("SubjectType = %q, want %q", claims.SubjectType, "APP")
	}
}

func TestJWKSResolverRefreshesRemoteJWKSWhenSignatureUsesRotatedKey(t *testing.T) {
	t.Parallel()

	oldKey := newTestJWKSRSAKey(t, "data-plane-key-rotated")
	newKey := newTestJWKSRSAKey(t, "data-plane-key-rotated")

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

	resolver, err := NewJWKSResolver(server.URL)
	if err != nil {
		t.Fatalf("NewJWKSResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer "+newKey.signedToken(t, map[string]any{
		"sub":          "user-001",
		"subject_type": "USER",
		"tenant_id":    "tenant-a",
		"scope":        []string{"file:read"},
		"iss":          "https://issuer.example.com",
		"aud":          []string{"zhi-file-data-plane"},
		"iat":          time.Now().Add(-time.Minute).Unix(),
		"exp":          time.Now().Add(time.Hour).Unix(),
	}))

	if _, err := resolver(req); err != nil {
		t.Fatalf("resolver() error = %v", err)
	}
	if requests.Load() < 2 {
		t.Fatalf("jwks requests = %d, want at least 2", requests.Load())
	}
}

func TestJWKSResolverRejectsFutureIssuedAt(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "data-plane-key-iat")
	resolver, err := NewJWKSResolver(key.jwksJSON(t))
	if err != nil {
		t.Fatalf("NewJWKSResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":          "user-001",
		"subject_type": "USER",
		"tenant_id":    "tenant-a",
		"scope":        "file:read",
		"iss":          "https://issuer.example.com",
		"aud":          "zhi-file-data-plane",
		"iat":          time.Now().Add(2 * time.Minute).Unix(),
		"exp":          time.Now().Add(time.Hour).Unix(),
	}))

	_, err = resolver(req)
	if code := xerrors.CodeOf(err); code != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeUnauthorized, err)
	}
}

func TestJWKSResolverRejectsMissingBearerToken(t *testing.T) {
	t.Parallel()

	key := newTestJWKSRSAKey(t, "data-plane-key-bearer")
	resolver, err := NewJWKSResolver(key.jwksJSON(t))
	if err != nil {
		t.Fatalf("NewJWKSResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)

	_, err = resolver(req)
	if code := xerrors.CodeOf(err); code != xerrors.CodeUnauthorized {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeUnauthorized, err)
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
	return testJWKSRSAKey{privateKey: privateKey, kid: kid}
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
