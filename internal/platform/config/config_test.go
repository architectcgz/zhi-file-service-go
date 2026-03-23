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

func TestLoadReturnsValidationErrorForMissingUploadAuthJWKS(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("UPLOAD_AUTH_JWKS", "")

	_, err := Load(ServiceUpload)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "UPLOAD_AUTH_JWKS") {
		t.Fatalf("expected UPLOAD_AUTH_JWKS error, got: %v", err)
	}
}

func TestLoadReturnsValidationErrorForMissingAccessAuthJWKS(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("ACCESS_AUTH_JWKS", "")

	_, err := Load(ServiceAccess)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "ACCESS_AUTH_JWKS") {
		t.Fatalf("expected ACCESS_AUTH_JWKS error, got: %v", err)
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

func TestLoadJobServiceParsesPhase6Intervals(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("JOB_PROCESS_OUTBOX_EVENTS_INTERVAL", "15s")
	t.Setenv("JOB_CLEANUP_MULTIPART_INTERVAL", "12m")

	cfg, err := Load(ServiceJob)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Job.ProcessOutboxEventsInterval != 15*time.Second {
		t.Fatalf("ProcessOutboxEventsInterval = %s, want %s", cfg.Job.ProcessOutboxEventsInterval, 15*time.Second)
	}
	if cfg.Job.CleanupMultipartInterval != 12*time.Minute {
		t.Fatalf("CleanupMultipartInterval = %s, want %s", cfg.Job.CleanupMultipartInterval, 12*time.Minute)
	}
}

func TestLoadAdminServiceParsesAllowedIssuers(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("ADMIN_AUTH_ALLOWED_ISSUERS", "https://issuer-a.example.com, https://issuer-b.example.com")

	cfg, err := Load(ServiceAdmin)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(cfg.Admin.AuthAllowedIssuers) != 2 {
		t.Fatalf("AuthAllowedIssuers len = %d, want 2", len(cfg.Admin.AuthAllowedIssuers))
	}
	if cfg.Admin.AuthAllowedIssuers[0] != "https://issuer-a.example.com" {
		t.Fatalf("AuthAllowedIssuers[0] = %q, want %q", cfg.Admin.AuthAllowedIssuers[0], "https://issuer-a.example.com")
	}
	if cfg.Admin.AuthAllowedIssuers[1] != "https://issuer-b.example.com" {
		t.Fatalf("AuthAllowedIssuers[1] = %q, want %q", cfg.Admin.AuthAllowedIssuers[1], "https://issuer-b.example.com")
	}
}

func TestLoadUploadServiceParsesAllowedIssuers(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("UPLOAD_AUTH_ALLOWED_ISSUERS", "https://issuer-a.example.com, https://issuer-b.example.com")

	cfg, err := Load(ServiceUpload)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(cfg.Upload.AuthAllowedIssuers) != 2 {
		t.Fatalf("Upload.AuthAllowedIssuers len = %d, want 2", len(cfg.Upload.AuthAllowedIssuers))
	}
	if cfg.Upload.AuthAllowedIssuers[0] != "https://issuer-a.example.com" {
		t.Fatalf("Upload.AuthAllowedIssuers[0] = %q, want %q", cfg.Upload.AuthAllowedIssuers[0], "https://issuer-a.example.com")
	}
	if cfg.Upload.AuthAllowedIssuers[1] != "https://issuer-b.example.com" {
		t.Fatalf("Upload.AuthAllowedIssuers[1] = %q, want %q", cfg.Upload.AuthAllowedIssuers[1], "https://issuer-b.example.com")
	}
}

func TestLoadAccessServiceParsesAllowedIssuers(t *testing.T) {
	setCommonEnv(t)
	t.Setenv("ACCESS_AUTH_ALLOWED_ISSUERS", "https://issuer-a.example.com, https://issuer-b.example.com")

	cfg, err := Load(ServiceAccess)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(cfg.Access.AuthAllowedIssuers) != 2 {
		t.Fatalf("Access.AuthAllowedIssuers len = %d, want 2", len(cfg.Access.AuthAllowedIssuers))
	}
	if cfg.Access.AuthAllowedIssuers[0] != "https://issuer-a.example.com" {
		t.Fatalf("Access.AuthAllowedIssuers[0] = %q, want %q", cfg.Access.AuthAllowedIssuers[0], "https://issuer-a.example.com")
	}
	if cfg.Access.AuthAllowedIssuers[1] != "https://issuer-b.example.com" {
		t.Fatalf("Access.AuthAllowedIssuers[1] = %q, want %q", cfg.Access.AuthAllowedIssuers[1], "https://issuer-b.example.com")
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
	t.Setenv("ACCESS_AUTH_JWKS", "jwks")
	t.Setenv("UPLOAD_AUTH_JWKS", "jwks")
	t.Setenv("ADMIN_AUTH_JWKS", "jwks")
}
