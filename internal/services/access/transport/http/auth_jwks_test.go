package httptransport

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestJWKSAuthResolverMapsClaimsToAccessAuthContext(t *testing.T) {
	t.Parallel()

	key := newAccessTestJWKSRSAKey(t, "access-key")
	resolver, err := NewJWKSAuthResolverWithIssuers(key.jwksJSON(t), []string{"https://issuer.example.com"})
	if err != nil {
		t.Fatalf("NewJWKSAuthResolverWithIssuers() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":          "user-001",
		"subject_type": "USER",
		"tenant_id":    "tenant-a",
		"scope":        "file:read",
		"iss":          "https://issuer.example.com",
		"aud":          []string{"zhi-file-data-plane"},
		"iat":          time.Now().Add(-time.Minute).Unix(),
		"exp":          time.Now().Add(time.Hour).Unix(),
		"client_id":    "client-001",
		"jti":          "token-001",
	}))
	req.Header.Set("X-Request-Id", "req-access-1")

	auth, err := resolver(req)
	if err != nil {
		t.Fatalf("resolver() error = %v", err)
	}
	if auth.RequestID != "req-access-1" {
		t.Fatalf("RequestID = %q, want %q", auth.RequestID, "req-access-1")
	}
	if auth.SubjectID != "user-001" || auth.SubjectType != "USER" {
		t.Fatalf("unexpected subject context: %#v", auth)
	}
	if auth.TenantID != "tenant-a" {
		t.Fatalf("TenantID = %q, want %q", auth.TenantID, "tenant-a")
	}
	if auth.ClientID != "client-001" || auth.TokenID != "token-001" {
		t.Fatalf("unexpected client/token context: %#v", auth)
	}
	if len(auth.Scopes) != 1 || auth.Scopes[0] != domain.ScopeFileRead {
		t.Fatalf("Scopes = %#v, want [%q]", auth.Scopes, domain.ScopeFileRead)
	}
}

func TestJWKSAuthResolverRejectsInvalidClaims(t *testing.T) {
	t.Parallel()

	key := newAccessTestJWKSRSAKey(t, "access-key-invalid")
	resolver, err := NewJWKSAuthResolver(key.jwksJSON(t))
	if err != nil {
		t.Fatalf("NewJWKSAuthResolver() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.Header.Set("Authorization", "Bearer "+key.signedToken(t, map[string]any{
		"sub":          "user-001",
		"subject_type": "USER",
		"tenant_id":    "tenant-a",
		"scope":        []string{"file:read"},
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

type accessTestJWKSRSAKey struct {
	privateKey *rsa.PrivateKey
	kid        string
}

func newAccessTestJWKSRSAKey(t *testing.T, kid string) accessTestJWKSRSAKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	return accessTestJWKSRSAKey{privateKey: privateKey, kid: kid}
}

func (k accessTestJWKSRSAKey) jwksJSON(t *testing.T) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": k.kid,
				"use": "sig",
				"alg": "RS256",
				"n":   accessBase64RawURLEncode(k.privateKey.PublicKey.N.Bytes()),
				"e":   accessBase64RawURLEncode(big.NewInt(int64(k.privateKey.PublicKey.E)).Bytes()),
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(payload)
}

func (k accessTestJWKSRSAKey) signedToken(t *testing.T, claims map[string]any) string {
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

	signingInput := accessBase64RawURLEncode(headerJSON) + "." + accessBase64RawURLEncode(claimsJSON)
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, k.privateKey, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("rsa.SignPKCS1v15() error = %v", err)
	}

	return signingInput + "." + accessBase64RawURLEncode(signature)
}

func accessBase64RawURLEncode(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
