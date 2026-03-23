package contract_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accesscommands "github.com/architectcgz/zhi-file-service-go/internal/services/access/app/commands"
	accessqueries "github.com/architectcgz/zhi-file-service-go/internal/services/access/app/queries"
	accessdomain "github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	accesshttp "github.com/architectcgz/zhi-file-service-go/internal/services/access/transport/http"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestAccessServiceGetFileContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(accesshttp.NewHandler(accesshttp.Options{
		Auth: func(*http.Request) (accessdomain.AuthContext, error) {
			return accessdomain.AuthContext{
				SubjectID: "user-1",
				TenantID:  "tenant-a",
				Scopes:    []string{accessdomain.ScopeFileRead},
			}, nil
		},
		GetFile: accessGetFileFunc(func(context.Context, accessqueries.GetFileQuery) (accessqueries.GetFileResult, error) {
			now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
			return accessqueries.GetFileResult{
				File: accessdomain.FileView{
					FileID:          "file-1",
					TenantID:        "tenant-a",
					FileName:        "avatar.png",
					ContentType:     "image/png",
					SizeBytes:       182044,
					AccessLevel:     pkgstorage.AccessLevelPublic,
					Status:          accessdomain.FileStatusActive,
					StorageProvider: pkgstorage.ProviderS3,
					BucketName:      "public-bucket",
					ObjectKey:       "tenant-a/avatar.png",
					CreatedAt:       now,
					UpdatedAt:       now,
				},
				DownloadURL: "https://cdn.example.com/public/tenant-a/avatar.png",
			}, nil
		}),
	}))
	defer server.Close()

	req := newRequest(t, http.MethodGet, server.URL+"/api/v1/files/file-1", "")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Request-Id", "req-access-contract-1")

	resp := doRequest(t, server.Client(), req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	payload := decodeJSONResponse(t, resp.Body)
	data := payload["data"].(map[string]any)
	if data["fileId"] != "file-1" {
		t.Fatalf("fileId = %v, want %q", data["fileId"], "file-1")
	}
	if data["downloadUrl"] != "https://cdn.example.com/public/tenant-a/avatar.png" {
		t.Fatalf("downloadUrl = %v, want expected public url", data["downloadUrl"])
	}
}

func TestAccessServiceRejectsMissingAuthContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(accesshttp.NewHandler(accesshttp.Options{
		GetFile: accessGetFileFunc(func(context.Context, accessqueries.GetFileQuery) (accessqueries.GetFileResult, error) {
			return accessqueries.GetFileResult{}, nil
		}),
	}))
	defer server.Close()

	req := newRequest(t, http.MethodGet, server.URL+"/api/v1/files/file-1", "")
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

func TestAccessServiceTicketRedirectContract(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	server := httptest.NewServer(accesshttp.NewHandler(accesshttp.Options{
		RedirectByAccessTicket: accessRedirectByTicketFunc(func(context.Context, accessqueries.RedirectByAccessTicketQuery) (accessqueries.RedirectByAccessTicketResult, error) {
			now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
			return accessqueries.RedirectByAccessTicketResult{
				File: accessdomain.FileView{
					FileID:          "file-1",
					TenantID:        "tenant-a",
					FileName:        "avatar.png",
					ContentType:     "image/png",
					SizeBytes:       182044,
					AccessLevel:     pkgstorage.AccessLevelPrivate,
					Status:          accessdomain.FileStatusActive,
					StorageProvider: pkgstorage.ProviderS3,
					BucketName:      "private-bucket",
					ObjectKey:       "tenant-a/avatar.png",
					CreatedAt:       now,
					UpdatedAt:       now,
				},
				URL:   "https://storage.example.com/private/file-1?sig=1",
				Claim: accessdomain.AccessTicketClaims{FileID: "file-1"},
			}, nil
		}),
	}))
	defer server.Close()

	req := newRequest(t, http.MethodGet, server.URL+"/api/v1/access-tickets/at_1/redirect", "")
	resp := doRequest(t, client, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	if got := resp.Header.Get("Location"); got != "https://storage.example.com/private/file-1?sig=1" {
		t.Fatalf("Location = %q, want redirect url", got)
	}
}

type accessGetFileFunc func(context.Context, accessqueries.GetFileQuery) (accessqueries.GetFileResult, error)

func (fn accessGetFileFunc) Handle(ctx context.Context, query accessqueries.GetFileQuery) (accessqueries.GetFileResult, error) {
	return fn(ctx, query)
}

type accessCreateTicketFunc func(context.Context, accesscommands.CreateAccessTicketCommand) (accesscommands.CreateAccessTicketResult, error)

func (fn accessCreateTicketFunc) Handle(ctx context.Context, command accesscommands.CreateAccessTicketCommand) (accesscommands.CreateAccessTicketResult, error) {
	return fn(ctx, command)
}

type accessRedirectByTicketFunc func(context.Context, accessqueries.RedirectByAccessTicketQuery) (accessqueries.RedirectByAccessTicketResult, error)

func (fn accessRedirectByTicketFunc) Handle(ctx context.Context, query accessqueries.RedirectByAccessTicketQuery) (accessqueries.RedirectByAccessTicketResult, error) {
	return fn(ctx, query)
}

