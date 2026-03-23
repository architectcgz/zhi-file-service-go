package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/observability"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestCreateUploadSessionWritesCreatedResponse(t *testing.T) {
	t.Parallel()

	var got commands.CreateUploadSessionCommand
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		CreateUploadSession: createUploadSessionFunc(func(_ context.Context, command commands.CreateUploadSessionCommand) (view.UploadSession, error) {
			got = command
			return sampleUploadSession(domain.SessionModePresignedSingle, domain.SessionStatusInitiated), nil
		}),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload-sessions", bytes.NewBufferString(`{
		"fileName":"avatar.png",
		"contentType":"image/png",
		"sizeBytes":182044,
		"accessLevel":"PUBLIC",
		"uploadMode":"PRESIGNED_SINGLE",
		"contentHash":{"algorithm":"SHA256","value":"4f6f0d53c1efb6bb7c9f6b4e5b7d7e2b7b5b2f4b33f3ef0d4ec2ef9f74de4f75"},
		"metadata":{"bizType":"avatar"}
	}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-create-1")
	req.Header.Set("Idempotency-Key", "create-key-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if got.IdempotencyKey != "create-key-1" {
		t.Fatalf("IdempotencyKey = %q, want %q", got.IdempotencyKey, "create-key-1")
	}
	if got.FileName != "avatar.png" || got.ContentType != "image/png" {
		t.Fatalf("unexpected create command: %#v", got)
	}
	if got.AccessLevel != pkgstorage.AccessLevelPublic {
		t.Fatalf("AccessLevel = %q, want %q", got.AccessLevel, pkgstorage.AccessLevelPublic)
	}
	if got.UploadMode != domain.SessionModePresignedSingle {
		t.Fatalf("UploadMode = %q, want %q", got.UploadMode, domain.SessionModePresignedSingle)
	}
	if got.Auth.RequestID != "req-create-1" {
		t.Fatalf("Auth.RequestID = %q, want %q", got.Auth.RequestID, "req-create-1")
	}

	payload := decodeJSONResponse(t, rr.Body.Bytes())
	if payload["requestId"] != "req-create-1" {
		t.Fatalf("requestId = %v, want %q", payload["requestId"], "req-create-1")
	}
	data := payload["data"].(map[string]any)
	if data["uploadSessionId"] != "upload-1" {
		t.Fatalf("uploadSessionId = %v, want %q", data["uploadSessionId"], "upload-1")
	}
}

func TestGetUploadSessionWritesResponse(t *testing.T) {
	t.Parallel()

	var got queries.GetUploadSessionQuery
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		GetUploadSession: getUploadSessionFunc(func(_ context.Context, query queries.GetUploadSessionQuery) (view.UploadSession, error) {
			got = query
			return sampleUploadSession(domain.SessionModeInline, domain.SessionStatusUploading), nil
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload-sessions/upload-1", nil)
	req.SetPathValue("uploadSessionId", "upload-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Request-Id", "req-get-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got.UploadSessionID != "upload-1" {
		t.Fatalf("UploadSessionID = %q, want %q", got.UploadSessionID, "upload-1")
	}
}

func TestUploadInlineContentWritesResponse(t *testing.T) {
	t.Parallel()

	var got commands.UploadInlineContentCommand
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		UploadInlineContent: uploadInlineContentFunc(func(_ context.Context, command commands.UploadInlineContentCommand) (view.UploadSession, error) {
			got = command
			return sampleUploadSession(domain.SessionModeInline, domain.SessionStatusUploading), nil
		}),
	})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/upload-sessions/upload-1/content", bytes.NewBufferString("payload"))
	req.SetPathValue("uploadSessionId", "upload-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Request-Id", "req-inline-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got.UploadSessionID != "upload-1" {
		t.Fatalf("UploadSessionID = %q, want %q", got.UploadSessionID, "upload-1")
	}
	if got.ContentType != "application/octet-stream" {
		t.Fatalf("ContentType = %q, want %q", got.ContentType, "application/octet-stream")
	}
	if got.Body == nil {
		t.Fatal("Body = nil, want non-nil")
	}
}

func TestPresignMultipartPartsWritesResponse(t *testing.T) {
	t.Parallel()

	var got commands.PresignMultipartPartsCommand
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		PresignMultipartParts: presignMultipartPartsFunc(func(_ context.Context, command commands.PresignMultipartPartsCommand) (commands.PresignMultipartPartsResult, error) {
			got = command
			return commands.PresignMultipartPartsResult{
				Parts: []view.PresignedPart{
					{PartNumber: 1, URL: "https://example.com/1", ExpiresAt: time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)},
				},
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload-sessions/upload-1/parts/presign", bytes.NewBufferString(`{
		"expiresInSeconds":900,
		"parts":[{"partNumber":1},{"partNumber":2}]
	}`))
	req.SetPathValue("uploadSessionId", "upload-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-presign-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got.ExpiresIn != 15*time.Minute {
		t.Fatalf("ExpiresIn = %s, want %s", got.ExpiresIn, 15*time.Minute)
	}
	if len(got.Parts) != 2 || got.Parts[0].PartNumber != 1 || got.Parts[1].PartNumber != 2 {
		t.Fatalf("unexpected parts: %#v", got.Parts)
	}
}

func TestListUploadedPartsWritesResponse(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		ListUploadedParts: listUploadedPartsFunc(func(_ context.Context, query queries.ListUploadedPartsQuery) (queries.ListUploadedPartsResult, error) {
			if query.UploadSessionID != "upload-1" {
				t.Fatalf("UploadSessionID = %q, want %q", query.UploadSessionID, "upload-1")
			}
			return queries.ListUploadedPartsResult{
				Parts: []view.UploadedPart{
					{PartNumber: 1, ETag: `"etag-1"`, SizeBytes: 1024},
				},
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload-sessions/upload-1/parts", nil)
	req.SetPathValue("uploadSessionId", "upload-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Request-Id", "req-parts-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	payload := decodeJSONResponse(t, rr.Body.Bytes())
	data := payload["data"].(map[string]any)
	parts := data["parts"].([]any)
	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
}

func TestCompleteUploadSessionWritesResponse(t *testing.T) {
	t.Parallel()

	var got commands.CompleteUploadSessionCommand
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		CompleteUploadSession: completeUploadSessionFunc(func(_ context.Context, command commands.CompleteUploadSessionCommand) (view.CompletedUploadSession, error) {
			got = command
			return view.CompletedUploadSession{
				FileID:        "file-1",
				UploadSession: sampleUploadSession(domain.SessionModeDirect, domain.SessionStatusCompleted),
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload-sessions/upload-1/complete", bytes.NewBufferString(`{
		"uploadedParts":[{"partNumber":1,"etag":"\"etag-1\"","sizeBytes":1024}],
		"contentHash":{"algorithm":"SHA256","value":"4f6f0d53c1efb6bb7c9f6b4e5b7d7e2b7b5b2f4b33f3ef0d4ec2ef9f74de4f75"}
	}`))
	req.SetPathValue("uploadSessionId", "upload-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-complete-1")
	req.Header.Set("Idempotency-Key", "complete-key-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got.UploadSessionID != "upload-1" {
		t.Fatalf("UploadSessionID = %q, want %q", got.UploadSessionID, "upload-1")
	}
	if got.IdempotencyKey != "complete-key-1" || got.RequestID != "req-complete-1" {
		t.Fatalf("unexpected idempotency/request mapping: %#v", got)
	}
	if len(got.UploadedParts) != 1 || got.UploadedParts[0].PartNumber != 1 {
		t.Fatalf("unexpected uploaded parts: %#v", got.UploadedParts)
	}
}

func TestAbortUploadSessionWritesResponse(t *testing.T) {
	t.Parallel()

	var got commands.AbortUploadSessionCommand
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		AbortUploadSession: abortUploadSessionFunc(func(_ context.Context, command commands.AbortUploadSessionCommand) (view.UploadSession, error) {
			got = command
			return sampleUploadSession(domain.SessionModeInline, domain.SessionStatusAborted), nil
		}),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload-sessions/upload-1/abort", bytes.NewBufferString(`{"reason":"user cancelled"}`))
	req.SetPathValue("uploadSessionId", "upload-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-abort-1")
	req.Header.Set("Idempotency-Key", "abort-key-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got.Reason != "user cancelled" {
		t.Fatalf("Reason = %q, want %q", got.Reason, "user cancelled")
	}
	if got.IdempotencyKey != "abort-key-1" {
		t.Fatalf("IdempotencyKey = %q, want %q", got.IdempotencyKey, "abort-key-1")
	}
}

func TestHandlerRecordsUploadBusinessMetrics(t *testing.T) {
	t.Parallel()

	metrics := observability.NewMetrics(true)
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		Metrics: NewMetricsRecorder(metrics.Registry(), "upload-service"),
		CreateUploadSession: createUploadSessionFunc(func(_ context.Context, command commands.CreateUploadSessionCommand) (view.UploadSession, error) {
			return sampleUploadSession(command.UploadMode, domain.SessionStatusInitiated), nil
		}),
		CompleteUploadSession: completeUploadSessionFunc(func(_ context.Context, _ commands.CompleteUploadSessionCommand) (view.CompletedUploadSession, error) {
			time.Sleep(5 * time.Millisecond)
			return view.CompletedUploadSession{
				FileID:        "file-1",
				UploadSession: sampleUploadSession(domain.SessionModeDirect, domain.SessionStatusCompleted),
			}, nil
		}),
		AbortUploadSession: abortUploadSessionFunc(func(_ context.Context, _ commands.AbortUploadSessionCommand) (view.UploadSession, error) {
			return sampleUploadSession(domain.SessionModeInline, domain.SessionStatusAborted), nil
		}),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/upload-sessions", bytes.NewBufferString(`{
		"fileName":"avatar.png",
		"contentType":"image/png",
		"sizeBytes":182044,
		"accessLevel":"PUBLIC",
		"uploadMode":"PRESIGNED_SINGLE"
	}`))
	createReq.Header.Set("Authorization", "Bearer token")
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Request-Id", "req-metrics-create-1")
	createRes := httptest.NewRecorder()
	handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createRes.Code, createRes.Body.String())
	}

	completeReq := httptest.NewRequest(http.MethodPost, "/api/v1/upload-sessions/upload-1/complete", bytes.NewBufferString(`{
		"uploadedParts":[{"partNumber":1,"etag":"\"etag-1\"","sizeBytes":1024}]
	}`))
	completeReq.SetPathValue("uploadSessionId", "upload-1")
	completeReq.Header.Set("Authorization", "Bearer token")
	completeReq.Header.Set("Content-Type", "application/json")
	completeReq.Header.Set("X-Request-Id", "req-metrics-complete-1")
	completeRes := httptest.NewRecorder()
	handler.ServeHTTP(completeRes, completeReq)
	if completeRes.Code != http.StatusOK {
		t.Fatalf("unexpected complete status: %d body=%s", completeRes.Code, completeRes.Body.String())
	}

	abortReq := httptest.NewRequest(http.MethodPost, "/api/v1/upload-sessions/upload-1/abort", bytes.NewBufferString(`{"reason":"cancelled"}`))
	abortReq.SetPathValue("uploadSessionId", "upload-1")
	abortReq.Header.Set("Authorization", "Bearer token")
	abortReq.Header.Set("Content-Type", "application/json")
	abortReq.Header.Set("X-Request-Id", "req-metrics-abort-1")
	abortRes := httptest.NewRecorder()
	handler.ServeHTTP(abortRes, abortReq)
	if abortRes.Code != http.StatusOK {
		t.Fatalf("unexpected abort status: %d body=%s", abortRes.Code, abortRes.Body.String())
	}

	metricsBody := scrapeMetrics(t, metrics)
	for _, metricName := range []string{
		"upload_session_create_total",
		"upload_session_complete_total",
		"upload_session_abort_total",
		"upload_complete_duration_seconds_bucket",
	} {
		if !strings.Contains(metricsBody, metricName) {
			t.Fatalf("expected metric %q in output: %s", metricName, metricsBody)
		}
	}
}

func TestHandlerRecordsUploadCompleteFailureMetric(t *testing.T) {
	t.Parallel()

	metrics := observability.NewMetrics(true)
	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		Metrics: NewMetricsRecorder(metrics.Registry(), "upload-service"),
		CompleteUploadSession: completeUploadSessionFunc(func(context.Context, commands.CompleteUploadSessionCommand) (view.CompletedUploadSession, error) {
			return view.CompletedUploadSession{}, xerrors.New(domain.CodeUploadHashMismatch, "hash mismatch", nil)
		}),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload-sessions/upload-1/complete", bytes.NewBufferString(`{
		"uploadedParts":[{"partNumber":1,"etag":"\"etag-1\"","sizeBytes":1024}]
	}`))
	req.SetPathValue("uploadSessionId", "upload-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-complete-fail-1")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("unexpected complete failure status: %d body=%s", res.Code, res.Body.String())
	}

	metricsBody := scrapeMetrics(t, metrics)
	if !strings.Contains(metricsBody, `upload_session_complete_failed_total`) {
		t.Fatalf("expected upload_session_complete_failed_total metric, got: %s", metricsBody)
	}
	if !strings.Contains(metricsBody, `error_code="UPLOAD_HASH_MISMATCH"`) {
		t.Fatalf("expected UPLOAD_HASH_MISMATCH label, got: %s", metricsBody)
	}
}

func TestHandlerWritesCanonicalErrorResponse(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return domain.AuthContext{}, xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload-sessions/upload-1", nil)
	req.SetPathValue("uploadSessionId", "upload-1")
	req.Header.Set("Authorization", "Bearer broken")
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

func TestUploadInlineContentRejectsPayloadLargerThanConfiguredLimit(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Options{
		Auth: func(*http.Request) (domain.AuthContext, error) {
			return newTestAuth(), nil
		},
		MaxInlineBodyBytes: 4,
		UploadInlineContent: uploadInlineContentFunc(func(context.Context, commands.UploadInlineContentCommand) (view.UploadSession, error) {
			t.Fatal("UploadInlineContent should not be called")
			return view.UploadSession{}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/upload-sessions/upload-1/content", bytes.NewBufferString("payload"))
	req.SetPathValue("uploadSessionId", "upload-1")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Request-Id", "req-inline-limit-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
	}
}

func decodeJSONResponse(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%s", err, string(raw))
	}
	return payload
}

func scrapeMetrics(t *testing.T, metrics *observability.Metrics) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	res := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("unexpected metrics status: %d body=%s", res.Code, res.Body.String())
	}
	return res.Body.String()
}

func newTestAuth() domain.AuthContext {
	return domain.AuthContext{
		RequestID:   "req-test-1",
		SubjectID:   "user-1",
		SubjectType: "USER",
		TenantID:    "tenant-a",
		Scopes:      []string{domain.ScopeFileWrite},
	}
}

func sampleUploadSession(mode domain.SessionMode, status domain.SessionStatus) view.UploadSession {
	completedAt := time.Date(2026, 3, 22, 10, 5, 0, 0, time.UTC)
	value := view.UploadSession{
		UploadSessionID: "upload-1",
		TenantID:        "tenant-a",
		UploadMode:      mode,
		Status:          status,
		FileName:        "avatar.png",
		ContentType:     "image/png",
		SizeBytes:       182044,
		AccessLevel:     pkgstorage.AccessLevelPublic,
		TotalParts:      2,
		UploadedParts:   1,
		PutURL:          "https://example.com/upload",
		PutHeaders: map[string]string{
			"Content-Type": "image/png",
		},
		FileID:    "file-1",
		CreatedAt: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 22, 10, 1, 0, 0, time.UTC),
	}
	if status == domain.SessionStatusCompleted {
		value.CompletedAt = &completedAt
		value.UploadedParts = 2
	}
	return value
}

type createUploadSessionFunc func(context.Context, commands.CreateUploadSessionCommand) (view.UploadSession, error)

func (f createUploadSessionFunc) Handle(ctx context.Context, command commands.CreateUploadSessionCommand) (view.UploadSession, error) {
	return f(ctx, command)
}

type getUploadSessionFunc func(context.Context, queries.GetUploadSessionQuery) (view.UploadSession, error)

func (f getUploadSessionFunc) Handle(ctx context.Context, query queries.GetUploadSessionQuery) (view.UploadSession, error) {
	return f(ctx, query)
}

type uploadInlineContentFunc func(context.Context, commands.UploadInlineContentCommand) (view.UploadSession, error)

func (f uploadInlineContentFunc) Handle(ctx context.Context, command commands.UploadInlineContentCommand) (view.UploadSession, error) {
	return f(ctx, command)
}

type presignMultipartPartsFunc func(context.Context, commands.PresignMultipartPartsCommand) (commands.PresignMultipartPartsResult, error)

func (f presignMultipartPartsFunc) Handle(ctx context.Context, command commands.PresignMultipartPartsCommand) (commands.PresignMultipartPartsResult, error) {
	return f(ctx, command)
}

type listUploadedPartsFunc func(context.Context, queries.ListUploadedPartsQuery) (queries.ListUploadedPartsResult, error)

func (f listUploadedPartsFunc) Handle(ctx context.Context, query queries.ListUploadedPartsQuery) (queries.ListUploadedPartsResult, error) {
	return f(ctx, query)
}

type completeUploadSessionFunc func(context.Context, commands.CompleteUploadSessionCommand) (view.CompletedUploadSession, error)

func (f completeUploadSessionFunc) Handle(ctx context.Context, command commands.CompleteUploadSessionCommand) (view.CompletedUploadSession, error) {
	return f(ctx, command)
}

type abortUploadSessionFunc func(context.Context, commands.AbortUploadSessionCommand) (view.UploadSession, error)

func (f abortUploadSessionFunc) Handle(ctx context.Context, command commands.AbortUploadSessionCommand) (view.UploadSession, error) {
	return f(ctx, command)
}

var _ io.Reader
