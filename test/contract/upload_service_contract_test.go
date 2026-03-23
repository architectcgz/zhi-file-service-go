package contract_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	uploadcommands "github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	uploadview "github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	uploaddomain "github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	uploadhttp "github.com/architectcgz/zhi-file-service-go/internal/services/upload/transport/http"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestUploadServiceCreateUploadSessionContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(uploadhttp.NewHandler(uploadhttp.Options{
		Auth: func(*http.Request) (uploaddomain.AuthContext, error) {
			return uploaddomain.AuthContext{
				SubjectID: "user-1",
				TenantID:  "tenant-a",
				Scopes:    []string{uploaddomain.ScopeFileWrite},
			}, nil
		},
		CreateUploadSession: uploadCreateUploadSessionFunc(func(context.Context, uploadcommands.CreateUploadSessionCommand) (uploadview.UploadSession, error) {
			now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
			return uploadview.UploadSession{
				UploadSessionID: "upload-1",
				TenantID:        "tenant-a",
				UploadMode:      uploaddomain.SessionModePresignedSingle,
				Status:          uploaddomain.SessionStatusInitiated,
				FileName:        "avatar.png",
				ContentType:     "image/png",
				SizeBytes:       182044,
				AccessLevel:     pkgstorage.AccessLevelPublic,
				TotalParts:      1,
				UploadedParts:   0,
				PutURL:          "https://storage.example.com/upload-1",
				PutHeaders: map[string]string{
					"Content-Type": "image/png",
				},
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		}),
	}))
	defer server.Close()

	req := newRequest(t, http.MethodPost, server.URL+"/api/v1/upload-sessions", `{
		"fileName":"avatar.png",
		"contentType":"image/png",
		"sizeBytes":182044,
		"accessLevel":"PUBLIC",
		"uploadMode":"PRESIGNED_SINGLE",
		"contentHash":{"algorithm":"SHA256","value":"4f6f0d53c1efb6bb7c9f6b4e5b7d7e2b7b5b2f4b33f3ef0d4ec2ef9f74de4f75"}
	}`)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-upload-contract-1")
	req.Header.Set("Idempotency-Key", "upload-contract-1")

	resp := doRequest(t, server.Client(), req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := resp.Header.Get("X-Request-Id"); got != "req-upload-contract-1" {
		t.Fatalf("X-Request-Id = %q, want %q", got, "req-upload-contract-1")
	}

	payload := decodeJSONResponse(t, resp.Body)
	if payload["requestId"] != "req-upload-contract-1" {
		t.Fatalf("requestId = %v, want %q", payload["requestId"], "req-upload-contract-1")
	}
	data := payload["data"].(map[string]any)
	if data["uploadSessionId"] != "upload-1" {
		t.Fatalf("uploadSessionId = %v, want %q", data["uploadSessionId"], "upload-1")
	}
	if data["putUrl"] != "https://storage.example.com/upload-1" {
		t.Fatalf("putUrl = %v, want expected presigned url", data["putUrl"])
	}
}

func TestUploadServiceRejectsInvalidBodyContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(uploadhttp.NewHandler(uploadhttp.Options{
		Auth: func(*http.Request) (uploaddomain.AuthContext, error) {
			return uploaddomain.AuthContext{
				SubjectID: "user-1",
				TenantID:  "tenant-a",
				Scopes:    []string{uploaddomain.ScopeFileWrite},
			}, nil
		},
		CreateUploadSession: uploadCreateUploadSessionFunc(func(context.Context, uploadcommands.CreateUploadSessionCommand) (uploadview.UploadSession, error) {
			return uploadview.UploadSession{}, nil
		}),
	}))
	defer server.Close()

	req := newRequest(t, http.MethodPost, server.URL+"/api/v1/upload-sessions", `{"unknown":true}`)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-upload-contract-2")

	resp := doRequest(t, server.Client(), req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	payload := decodeJSONResponse(t, resp.Body)
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != string(xerrors.CodeInvalidArgument) {
		t.Fatalf("error.code = %v, want %q", errPayload["code"], xerrors.CodeInvalidArgument)
	}
}

type uploadCreateUploadSessionFunc func(context.Context, uploadcommands.CreateUploadSessionCommand) (uploadview.UploadSession, error)

func (fn uploadCreateUploadSessionFunc) Handle(ctx context.Context, command uploadcommands.CreateUploadSessionCommand) (uploadview.UploadSession, error) {
	return fn(ctx, command)
}

