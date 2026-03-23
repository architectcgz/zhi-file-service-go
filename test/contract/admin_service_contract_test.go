package contract_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	admincommands "github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/commands"
	adminqueries "github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/queries"
	adminview "github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	admindomain "github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	adminhttp "github.com/architectcgz/zhi-file-service-go/internal/services/admin/transport/http"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestAdminServiceCreateTenantContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(adminhttp.NewHandler(adminhttp.Options{
		Auth: func(*http.Request) (admindomain.AdminContext, error) {
			return newAdminContext(t, admindomain.RoleSuper, admindomain.GlobalTenantScope), nil
		},
		CreateTenant: adminCreateTenantFunc(func(context.Context, admincommands.CreateTenantCommand) (adminview.Tenant, error) {
			now := adminview.Time(time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC))
			return adminview.Tenant{
				TenantID:     "tenant-a",
				TenantName:   "Tenant A",
				Status:       admindomain.TenantStatusActive,
				ContactEmail: "ops@example.com",
				Description:  "contract test tenant",
				CreatedAt:    now,
				UpdatedAt:    now,
			}, nil
		}),
	}))
	defer server.Close()

	req := newRequest(t, http.MethodPost, server.URL+"/api/admin/v1/tenants", `{
		"tenantId":"tenant-a",
		"tenantName":"Tenant A",
		"contactEmail":"ops@example.com",
		"description":"contract test tenant"
	}`)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-admin-contract-1")
	req.Header.Set("Idempotency-Key", "admin-contract-1")

	resp := doRequest(t, server.Client(), req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	payload := decodeJSONResponse(t, resp.Body)
	data := payload["data"].(map[string]any)
	if data["tenantId"] != "tenant-a" {
		t.Fatalf("tenantId = %v, want %q", data["tenantId"], "tenant-a")
	}
	if data["status"] != string(admindomain.TenantStatusActive) {
		t.Fatalf("status = %v, want %q", data["status"], admindomain.TenantStatusActive)
	}
}

func TestAdminServiceRejectsMissingAuthContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(adminhttp.NewHandler(adminhttp.Options{
		GetTenant: adminGetTenantFunc(func(context.Context, adminqueries.GetTenantQuery) (adminview.Tenant, error) {
			return adminview.Tenant{}, nil
		}),
	}))
	defer server.Close()

	req := newRequest(t, http.MethodGet, server.URL+"/api/admin/v1/tenants/tenant-a", "")
	resp := doRequest(t, server.Client(), req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	payload := decodeJSONResponse(t, resp.Body)
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != string(xerrors.CodeUnauthorized) {
		t.Fatalf("error.code = %v, want %q", errPayload["code"], xerrors.CodeUnauthorized)
	}
}

func newAdminContext(t *testing.T, role admindomain.Role, scopes ...string) admindomain.AdminContext {
	t.Helper()

	auth, err := admindomain.NewAdminContext(admindomain.AdminContextInput{
		AdminID:      "admin-1",
		Roles:        []string{string(role)},
		TenantScopes: scopes,
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}
	return auth
}

type adminCreateTenantFunc func(context.Context, admincommands.CreateTenantCommand) (adminview.Tenant, error)

func (fn adminCreateTenantFunc) Handle(ctx context.Context, command admincommands.CreateTenantCommand) (adminview.Tenant, error) {
	return fn(ctx, command)
}

type adminGetTenantFunc func(context.Context, adminqueries.GetTenantQuery) (adminview.Tenant, error)

func (fn adminGetTenantFunc) Handle(ctx context.Context, query adminqueries.GetTenantQuery) (adminview.Tenant, error) {
	return fn(ctx, query)
}

