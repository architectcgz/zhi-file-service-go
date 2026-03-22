package queries_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

func TestGetUploadSessionReturnsBoundSession(t *testing.T) {
	now := time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC)
	session := mustQuerySession(t, domain.CreateSessionParams{
		ID:          "01JQ327CN3HDEYMX78S2AKK6YX",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "invoice.pdf",
		ContentType: "application/pdf",
		SizeBytes:   2048,
		AccessLevel: storage.AccessLevelPrivate,
		Mode:        domain.SessionModeInline,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/01JQ327CN3HDEYMX78S2AKK6YX/invoice.pdf"},
		CreatedAt:   now,
		UpdatedAt:   now.Add(5 * time.Second),
		ExpiresAt:   now.Add(30 * time.Minute),
	})
	handler := queries.NewGetUploadSessionHandler(&stubQuerySessionRepository{session: session})

	result, err := handler.Handle(context.Background(), queries.GetUploadSessionQuery{
		UploadSessionID: session.ID,
		Auth:            newQueryAuth(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.UploadSessionID != session.ID {
		t.Fatalf("expected upload session id %q, got %q", session.ID, result.UploadSessionID)
	}
	if result.Status != domain.SessionStatusInitiated {
		t.Fatalf("unexpected status: %s", result.Status)
	}
}

func TestListUploadedPartsReturnsStoredPartsForNonDirectSession(t *testing.T) {
	now := time.Date(2026, 3, 22, 11, 10, 0, 0, time.UTC)
	session := mustQuerySession(t, domain.CreateSessionParams{
		ID:          "01JQ329TZEY2H0QZBPB01W4J5X",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   1024,
		AccessLevel: storage.AccessLevelPublic,
		Mode:        domain.SessionModePresignedSingle,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "public-bucket", ObjectKey: "tenant-a/uploads/01JQ329TZEY2H0QZBPB01W4J5X/avatar.png"},
		Hash:        &domain.ContentHash{Algorithm: "SHA256", Value: validQueryHash()},
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(30 * time.Minute),
	})
	parts := &stubSessionPartRepository{
		records: []ports.SessionPartRecord{
			{UploadSessionID: session.ID, PartNumber: 1, ETag: "etag-1", PartSize: 1024},
		},
	}
	multipart := &stubQueryMultipartManager{}
	handler := queries.NewListUploadedPartsHandler(
		&stubQuerySessionRepository{session: session},
		parts,
		multipart,
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), queries.ListUploadedPartsQuery{
		UploadSessionID: session.ID,
		Auth:            newQueryAuth(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(result.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(result.Parts))
	}
	if result.Parts[0].SizeBytes != 1024 {
		t.Fatalf("unexpected part size: %d", result.Parts[0].SizeBytes)
	}
	if multipart.listCalls != 0 {
		t.Fatalf("expected no multipart list calls, got %d", multipart.listCalls)
	}
	if parts.replaceCalls != 0 {
		t.Fatalf("expected no part replacement, got %d", parts.replaceCalls)
	}
}

func TestListUploadedPartsRefreshesProviderStateForDirectSession(t *testing.T) {
	now := time.Date(2026, 3, 22, 11, 20, 0, 0, time.UTC)
	session := mustQuerySession(t, domain.CreateSessionParams{
		ID:               "01JQ32D6ZW1F8N5KXGQ8HGRV5F",
		TenantID:         "tenant-a",
		OwnerID:          "user-1",
		FileName:         "video.mp4",
		ContentType:      "video/mp4",
		SizeBytes:        734003200,
		AccessLevel:      storage.AccessLevelPrivate,
		Mode:             domain.SessionModeDirect,
		TotalParts:       3,
		Object:           storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/01JQ32D6ZW1F8N5KXGQ8HGRV5F/video.mp4"},
		ProviderUploadID: "upload-123",
		Hash:             &domain.ContentHash{Algorithm: "SHA256", Value: validQueryHash()},
		CreatedAt:        now,
		UpdatedAt:        now,
		ExpiresAt:        now.Add(30 * time.Minute),
	})
	parts := &stubSessionPartRepository{}
	multipart := &stubQueryMultipartManager{
		parts: []storage.UploadedPart{
			{PartNumber: 1, ETag: "etag-1", SizeBytes: 5242880, Checksum: "c1"},
			{PartNumber: 2, ETag: "etag-2", SizeBytes: 5242880, Checksum: "c2"},
		},
	}
	handler := queries.NewListUploadedPartsHandler(
		&stubQuerySessionRepository{session: session},
		parts,
		multipart,
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), queries.ListUploadedPartsQuery{
		UploadSessionID: session.ID,
		Auth:            newQueryAuth(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if multipart.listCalls != 1 {
		t.Fatalf("expected one multipart list call, got %d", multipart.listCalls)
	}
	if parts.replaceCalls != 1 {
		t.Fatalf("expected one part replacement, got %d", parts.replaceCalls)
	}
	if len(parts.replaced) != 2 {
		t.Fatalf("expected two replaced parts, got %d", len(parts.replaced))
	}
	if parts.replaced[0].UploadedAt != now {
		t.Fatalf("expected uploaded at %s, got %s", now, parts.replaced[0].UploadedAt)
	}
	if len(result.Parts) != 2 || result.Parts[1].PartNumber != 2 {
		t.Fatalf("unexpected query result: %#v", result.Parts)
	}
}

type stubQuerySessionRepository struct {
	session *domain.Session
	err     error
}

func (s *stubQuerySessionRepository) Create(context.Context, *domain.Session) error {
	panic("unexpected call")
}

func (s *stubQuerySessionRepository) Save(context.Context, *domain.Session) error {
	s.session = s.session
	return nil
}

func (s *stubQuerySessionRepository) GetByID(_ context.Context, _ string, _ string) (*domain.Session, error) {
	return s.session, s.err
}

func (s *stubQuerySessionRepository) FindReusable(context.Context, ports.ReusableSessionQuery) (*domain.Session, error) {
	panic("unexpected call")
}

func (s *stubQuerySessionRepository) AcquireCompletion(context.Context, ports.CompletionAcquireRequest) (*ports.CompletionAcquireResult, error) {
	panic("unexpected call")
}

func (s *stubQuerySessionRepository) ConfirmCompletionOwner(context.Context, string, string, string) (*domain.Session, error) {
	panic("unexpected call")
}

type stubSessionPartRepository struct {
	records      []ports.SessionPartRecord
	replaced     []ports.SessionPartRecord
	replaceCalls int
}

func (s *stubSessionPartRepository) ListBySessionID(context.Context, string, string) ([]ports.SessionPartRecord, error) {
	return s.records, nil
}

func (s *stubSessionPartRepository) Upsert(context.Context, ports.SessionPartRecord) error {
	panic("unexpected call")
}

func (s *stubSessionPartRepository) Replace(_ context.Context, _ string, _ string, records []ports.SessionPartRecord) error {
	s.replaceCalls++
	s.replaced = append([]ports.SessionPartRecord(nil), records...)
	return nil
}

type stubQueryMultipartManager struct {
	parts     []storage.UploadedPart
	listCalls int
}

func (s *stubQueryMultipartManager) CreateMultipartUpload(context.Context, storage.ObjectRef, string) (string, error) {
	panic("unexpected call")
}

func (s *stubQueryMultipartManager) UploadPart(context.Context, storage.ObjectRef, string, int, io.Reader, int64) (string, error) {
	panic("unexpected call")
}

func (s *stubQueryMultipartManager) ListUploadedParts(_ context.Context, _ storage.ObjectRef, _ string) ([]storage.UploadedPart, error) {
	s.listCalls++
	return s.parts, nil
}

func (s *stubQueryMultipartManager) CompleteMultipartUpload(context.Context, storage.ObjectRef, string, []storage.UploadedPart) error {
	panic("unexpected call")
}

func (s *stubQueryMultipartManager) AbortMultipartUpload(context.Context, storage.ObjectRef, string) error {
	panic("unexpected call")
}

func newQueryAuth() domain.AuthContext {
	return domain.AuthContext{
		SubjectID: "user-1",
		TenantID:  "tenant-a",
		Scopes:    []string{domain.ScopeFileWrite},
	}
}

func validQueryHash() string {
	return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
}

func mustQuerySession(t *testing.T, params domain.CreateSessionParams) *domain.Session {
	t.Helper()

	session, err := domain.NewSession(params)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	return session
}
