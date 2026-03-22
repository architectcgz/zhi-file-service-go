package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestGetFileWritesResponse(t *testing.T) {
	var got queries.GetFileQuery
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		GetFile: getFileFunc(func(_ context.Context, query queries.GetFileQuery) (queries.GetFileResult, error) {
			got = query
			return queries.GetFileResult{
				File:        sampleFile(storage.AccessLevelPublic),
				DownloadURL: "https://cdn.example.com/public/tenant-a/avatar.png",
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.SetPathValue("fileId", "file-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Request-Id", "req-file-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got.FileID != "file-1" {
		t.Fatalf("FileID = %q, want %q", got.FileID, "file-1")
	}

	payload := decodeJSONResponse(t, rr.Body.Bytes())
	if payload["requestId"] != "req-file-1" {
		t.Fatalf("requestId = %v, want %q", payload["requestId"], "req-file-1")
	}
}

func TestCreateAccessTicketWritesCreatedResponse(t *testing.T) {
	var got commands.CreateAccessTicketCommand
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		CreateAccessTicket: createAccessTicketFunc(func(_ context.Context, command commands.CreateAccessTicketCommand) (commands.CreateAccessTicketResult, error) {
			got = command
			return commands.CreateAccessTicketResult{
				Ticket:      "at_ticket_1",
				RedirectURL: "/api/v1/access-tickets/at_ticket_1/redirect",
				ExpiresAt:   time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC),
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/file-1/access-tickets", bytes.NewBufferString(`{
		"expiresInSeconds":300,
		"responseDisposition":"attachment",
		"responseFileName":"invoice.pdf"
	}`))
	req.SetPathValue("fileId", "file-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "ticket-001")
	req.Header.Set("X-Request-Id", "req-ticket-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if got.FileID != "file-1" {
		t.Fatalf("FileID = %q, want %q", got.FileID, "file-1")
	}
	if got.IdempotencyKey != "ticket-001" {
		t.Fatalf("IdempotencyKey = %q, want %q", got.IdempotencyKey, "ticket-001")
	}
	if got.ExpiresIn != 5*time.Minute {
		t.Fatalf("ExpiresIn = %s, want %s", got.ExpiresIn, 5*time.Minute)
	}
	if got.Disposition != domain.DownloadDispositionAttachment {
		t.Fatalf("Disposition = %q, want %q", got.Disposition, domain.DownloadDispositionAttachment)
	}
}

func TestResolveDownloadWritesRedirect(t *testing.T) {
	var got queries.ResolveDownloadQuery
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		ResolveDownload: resolveDownloadFunc(func(_ context.Context, query queries.ResolveDownloadQuery) (queries.ResolveDownloadResult, error) {
			got = query
			return queries.ResolveDownloadResult{
				File: sampleFile(storage.AccessLevelPrivate),
				URL:  "https://storage.example.com/private-bucket/tenant-a/invoice.pdf?sig=1",
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1/download?disposition=inline", nil)
	req.SetPathValue("fileId", "file-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Request-Id", "req-download-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	if rr.Header().Get("Location") == "" {
		t.Fatal("expected Location header")
	}
	if got.Disposition != domain.DownloadDispositionInline {
		t.Fatalf("Disposition = %q, want %q", got.Disposition, domain.DownloadDispositionInline)
	}
}

func TestRedirectByAccessTicketDoesNotRequireBearerToken(t *testing.T) {
	var got queries.RedirectByAccessTicketQuery
	handler := NewHandler(Options{
		RedirectByAccessTicket: redirectByAccessTicketFunc(func(_ context.Context, query queries.RedirectByAccessTicketQuery) (queries.RedirectByAccessTicketResult, error) {
			got = query
			return queries.RedirectByAccessTicketResult{
				File:  sampleFile(storage.AccessLevelPrivate),
				URL:   "https://storage.example.com/private-bucket/tenant-a/invoice.pdf?sig=1",
				Claim: domain.AccessTicketClaims{FileID: "file-1"},
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/access-tickets/at_ticket_1/redirect", nil)
	req.SetPathValue("ticket", "at_ticket_1")
	req.Header.Set("X-Request-Id", "req-redirect-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	if got.Ticket != "at_ticket_1" {
		t.Fatalf("Ticket = %q, want %q", got.Ticket, "at_ticket_1")
	}
}

func TestGetFileRejectsMissingAuth(t *testing.T) {
	handler := NewHandler(Options{
		GetFile: getFileFunc(func(context.Context, queries.GetFileQuery) (queries.GetFileResult, error) {
			return queries.GetFileResult{}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/file-1", nil)
	req.SetPathValue("fileId", "file-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
	payload := decodeJSONResponse(t, rr.Body.Bytes())
	errorPayload := payload["error"].(map[string]any)
	if errorPayload["code"] != string(xerrors.CodeUnauthorized) {
		t.Fatalf("error code = %v, want %q", errorPayload["code"], xerrors.CodeUnauthorized)
	}
}

type getFileFunc func(context.Context, queries.GetFileQuery) (queries.GetFileResult, error)

func (fn getFileFunc) Handle(ctx context.Context, query queries.GetFileQuery) (queries.GetFileResult, error) {
	return fn(ctx, query)
}

type createAccessTicketFunc func(context.Context, commands.CreateAccessTicketCommand) (commands.CreateAccessTicketResult, error)

func (fn createAccessTicketFunc) Handle(ctx context.Context, command commands.CreateAccessTicketCommand) (commands.CreateAccessTicketResult, error) {
	return fn(ctx, command)
}

type resolveDownloadFunc func(context.Context, queries.ResolveDownloadQuery) (queries.ResolveDownloadResult, error)

func (fn resolveDownloadFunc) Handle(ctx context.Context, query queries.ResolveDownloadQuery) (queries.ResolveDownloadResult, error) {
	return fn(ctx, query)
}

type redirectByAccessTicketFunc func(context.Context, queries.RedirectByAccessTicketQuery) (queries.RedirectByAccessTicketResult, error)

func (fn redirectByAccessTicketFunc) Handle(ctx context.Context, query queries.RedirectByAccessTicketQuery) (queries.RedirectByAccessTicketResult, error) {
	return fn(ctx, query)
}

func sampleFile(level storage.AccessLevel) domain.FileView {
	now := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	return domain.FileView{
		FileID:          "file-1",
		TenantID:        "tenant-a",
		FileName:        "invoice.pdf",
		ContentType:     "application/pdf",
		SizeBytes:       1024,
		AccessLevel:     level,
		Status:          domain.FileStatusActive,
		StorageProvider: storage.ProviderS3,
		BucketName:      "private-bucket",
		ObjectKey:       "tenant-a/invoice.pdf",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func newTestAuth() domain.AuthContext {
	return domain.AuthContext{
		SubjectID: "user-1",
		TenantID:  "tenant-a",
		Scopes:    []string{domain.ScopeFileRead},
	}
}

func decodeJSONResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return payload
}
