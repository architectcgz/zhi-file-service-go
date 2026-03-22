package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadUploadServiceDefaults(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := Load(ServiceUpload)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.App.ServiceName != ServiceUpload {
		t.Fatalf("unexpected service name: %s", cfg.App.ServiceName)
	}
	if cfg.HTTP.Port != 8080 {
		t.Fatalf("unexpected default port: %d", cfg.HTTP.Port)
	}
	if cfg.App.ShutdownTimeout != 15*time.Second {
		t.Fatalf("unexpected shutdown timeout: %s", cfg.App.ShutdownTimeout)
	}
}

func TestLoadReturnsValidationErrorForMissingDBDSN(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("DB_DSN", "")

	_, err := Load(ServiceUpload)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "DB_DSN") {
		t.Fatalf("expected DB_DSN error, got: %v", err)
	}
}

func TestLoadReturnsValidationErrorForAccessSecret(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("ACCESS_TICKET_SIGNING_KEY", "")

	_, err := Load(ServiceAccess)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "ACCESS_TICKET_SIGNING_KEY") {
		t.Fatalf("expected ACCESS_TICKET_SIGNING_KEY error, got: %v", err)
	}
}

func TestLoadReturnsValidationErrorForUnsupportedUploadMode(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("UPLOAD_ALLOWED_MODES", "INLINE,UNKNOWN")

	_, err := Load(ServiceUpload)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "UPLOAD_ALLOWED_MODES") {
		t.Fatalf("expected upload mode validation error, got: %v", err)
	}
}

func TestLoadReturnsValidationErrorForInvalidJobLockWindow(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("JOB_LOCK_TTL", "10s")
	t.Setenv("JOB_LOCK_RENEW_INTERVAL", "10s")

	_, err := Load(ServiceJob)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "JOB_LOCK_RENEW_INTERVAL") {
		t.Fatalf("expected job lock validation error, got: %v", err)
	}
}

func setCommonEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("DB_DSN", "postgres://test:test@127.0.0.1:5432/test")
	t.Setenv("STORAGE_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("STORAGE_ACCESS_KEY", "key")
	t.Setenv("STORAGE_SECRET_KEY", "secret")
	t.Setenv("STORAGE_PUBLIC_BUCKET", "public")
	t.Setenv("STORAGE_PRIVATE_BUCKET", "private")
	t.Setenv("ACCESS_TICKET_SIGNING_KEY", "sign-key")
	t.Setenv("ADMIN_AUTH_JWKS", "jwks")
}
