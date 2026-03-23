package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestListFilesWritesResponse(t *testing.T) {
	t.Parallel()

	var got queries.ListFilesQuery
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AdminContext, error) {
			return newTestAdminAuth(t, domain.RoleReadonly, "tenant-a"), nil
		},
		ListFiles: listFilesFunc(func(_ context.Context, query queries.ListFilesQuery) (view.AdminFileList, error) {
			got = query
			return view.AdminFileList{
				Items: []view.AdminFile{
					{
						FileID:      "file-1",
						TenantID:    "tenant-a",
						FileName:    "report.pdf",
						ContentType: "application/pdf",
						SizeBytes:   1024,
						AccessLevel: pkgstorage.AccessLevelPrivate,
						Status:      "ACTIVE",
						CreatedAt:   view.Time(time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)),
						UpdatedAt:   view.Time(time.Date(2026, 3, 22, 10, 1, 0, 0, time.UTC)),
					},
				},
				NextCursor: "cursor-2",
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/files?tenantId=tenant-a&status=ACTIVE&cursor=cursor-1&limit=25", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Request-Id", "req-list-files-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got.TenantID != "tenant-a" || got.Status != "ACTIVE" || got.Cursor != "cursor-1" || got.Limit != 25 {
		t.Fatalf("unexpected list query: %#v", got)
	}

	payload := decodeJSONResponse(t, rr.Body.Bytes())
	if payload["requestId"] != "req-list-files-1" {
		t.Fatalf("requestId = %v, want %q", payload["requestId"], "req-list-files-1")
	}
	page := payload["page"].(map[string]any)
	if page["nextCursor"] != "cursor-2" {
		t.Fatalf("nextCursor = %v, want %q", page["nextCursor"], "cursor-2")
	}
}

func TestDeleteFileWritesResponse(t *testing.T) {
	t.Parallel()

	var got commands.DeleteFileCommand
	deletedAt := time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC)
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AdminContext, error) {
			return newTestAdminAuth(t, domain.RoleGovernance, "tenant-a"), nil
		},
		DeleteFile: deleteFileFunc(func(_ context.Context, command commands.DeleteFileCommand) (view.DeleteFileResult, error) {
			got = command
			return view.DeleteFileResult{
				FileID:                  "file-1",
				Status:                  "DELETED",
				DeletedAt:               &deletedAt,
				PhysicalDeleteScheduled: true,
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/v1/files/file-1", bytes.NewBufferString(`{"reason":"manual cleanup"}`))
	req.SetPathValue("fileId", "file-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "delete-key-1")
	req.Header.Set("X-Request-Id", "req-delete-file-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got.FileID != "file-1" || got.Reason != "manual cleanup" || got.IdempotencyKey != "delete-key-1" {
		t.Fatalf("unexpected delete command: %#v", got)
	}

	payload := decodeJSONResponse(t, rr.Body.Bytes())
	data := payload["data"].(map[string]any)
	if data["status"] != "DELETED" {
		t.Fatalf("status = %v, want %q", data["status"], "DELETED")
	}
}

func TestGetTenantRejectsMissingAuth(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Options{
		GetTenant: getTenantFunc(func(context.Context, queries.GetTenantQuery) (view.Tenant, error) {
			return view.Tenant{}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/tenants/tenant-a", nil)
	req.SetPathValue("tenantId", "tenant-a")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
	payload := decodeJSONResponse(t, rr.Body.Bytes())
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != string(xerrors.CodeUnauthorized) {
		t.Fatalf("error.code = %v, want %q", errPayload["code"], xerrors.CodeUnauthorized)
	}
}

type getTenantFunc func(context.Context, queries.GetTenantQuery) (view.Tenant, error)

func (fn getTenantFunc) Handle(ctx context.Context, query queries.GetTenantQuery) (view.Tenant, error) {
	return fn(ctx, query)
}

type listFilesFunc func(context.Context, queries.ListFilesQuery) (view.AdminFileList, error)

func (fn listFilesFunc) Handle(ctx context.Context, query queries.ListFilesQuery) (view.AdminFileList, error) {
	return fn(ctx, query)
}

type deleteFileFunc func(context.Context, commands.DeleteFileCommand) (view.DeleteFileResult, error)

func (fn deleteFileFunc) Handle(ctx context.Context, command commands.DeleteFileCommand) (view.DeleteFileResult, error) {
	return fn(ctx, command)
}

func newTestAdminAuth(t *testing.T, role domain.Role, scopes ...string) domain.AdminContext {
	t.Helper()

	auth, err := domain.NewAdminContext(domain.AdminContextInput{
		AdminID:      "admin-1",
		Roles:        []string{string(role)},
		TenantScopes: scopes,
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}
	return auth
}

func decodeJSONResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return payload
}
