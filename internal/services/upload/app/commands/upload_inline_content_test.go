package commands_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestUploadInlineContentMarksSessionUploading(t *testing.T) {
	now := time.Date(2026, 3, 22, 16, 0, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "01JQ5C6X1S7QX8NSJ3AT4ZWRCG",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "report.pdf",
		ContentType: "application/pdf",
		SizeBytes:   5,
		AccessLevel: storage.AccessLevelPrivate,
		Mode:        domain.SessionModeInline,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/01JQ5C6X1S7QX8NSJ3AT4ZWRCG/report.pdf"},
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(29 * time.Minute),
	})
	sessions := &stubInlineSessionRepository{session: session}
	writer := &stubInlineObjectWriter{}
	handler := commands.NewUploadInlineContentHandler(sessions, writer, clock.NewFixed(now))

	result, err := handler.Handle(context.Background(), commands.UploadInlineContentCommand{
		UploadSessionID: session.ID,
		ContentType:     "application/pdf",
		Body:            bytes.NewBufferString("hello"),
		Auth:            newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want 1", writer.calls)
	}
	if writer.size != 5 {
		t.Fatalf("writer size = %d, want 5", writer.size)
	}
	if sessions.saved == nil {
		t.Fatal("expected session to be saved")
	}
	if sessions.saved.Status != domain.SessionStatusUploading || sessions.saved.CompletedParts != 1 {
		t.Fatalf("unexpected saved session: %#v", sessions.saved)
	}
	if result.Status != domain.SessionStatusUploading || result.UploadedParts != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestUploadInlineContentRejectsNonInlineMode(t *testing.T) {
	now := time.Date(2026, 3, 22, 16, 10, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "01JQ5CAXQ4KJ0GNAJ51X7H2HH4",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   4,
		AccessLevel: storage.AccessLevelPublic,
		Mode:        domain.SessionModePresignedSingle,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "public-bucket", ObjectKey: "tenant-a/uploads/01JQ5CAXQ4KJ0GNAJ51X7H2HH4/avatar.png"},
		Hash:        &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(29 * time.Minute),
	})
	handler := commands.NewUploadInlineContentHandler(&stubInlineSessionRepository{session: session}, &stubInlineObjectWriter{}, clock.NewFixed(now))

	_, err := handler.Handle(context.Background(), commands.UploadInlineContentCommand{
		UploadSessionID: session.ID,
		ContentType:     "image/png",
		Body:            bytes.NewBufferString("data"),
		Auth:            newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != domain.CodeUploadModeInvalid {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, domain.CodeUploadModeInvalid, err)
	}
}

func TestUploadInlineContentWrapsStorageErrors(t *testing.T) {
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "01JQ5CCQ4S740R0QHS06ZX1H74",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "report.pdf",
		ContentType: "application/pdf",
		SizeBytes:   5,
		AccessLevel: storage.AccessLevelPrivate,
		Mode:        domain.SessionModeInline,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/01JQ5CCQ4S740R0QHS06ZX1H74/report.pdf"},
		CreatedAt:   time.Date(2026, 3, 22, 16, 20, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 3, 22, 16, 20, 0, 0, time.UTC),
		ExpiresAt:   time.Date(2026, 3, 22, 16, 50, 0, 0, time.UTC),
	})
	handler := commands.NewUploadInlineContentHandler(
		&stubInlineSessionRepository{session: session},
		&stubInlineObjectWriter{err: errors.New("s3 down")},
		clock.NewFixed(time.Date(2026, 3, 22, 16, 21, 0, 0, time.UTC)),
	)

	_, err := handler.Handle(context.Background(), commands.UploadInlineContentCommand{
		UploadSessionID: session.ID,
		Body:            bytes.NewBufferString("hello"),
		Auth:            newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != xerrors.CodeServiceUnavailable {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeServiceUnavailable, err)
	}
}

type stubInlineSessionRepository struct {
	session *domain.Session
	saved   *domain.Session
	err     error
	saveErr error
}

func (s *stubInlineSessionRepository) Create(context.Context, *domain.Session) error {
	panic("unexpected call")
}

func (s *stubInlineSessionRepository) Save(_ context.Context, session *domain.Session) error {
	s.saved = session
	return s.saveErr
}

func (s *stubInlineSessionRepository) GetByID(context.Context, string, string) (*domain.Session, error) {
	return s.session, s.err
}

func (s *stubInlineSessionRepository) FindReusable(context.Context, ports.ReusableSessionQuery) (*domain.Session, error) {
	panic("unexpected call")
}

func (s *stubInlineSessionRepository) AcquireCompletion(context.Context, ports.CompletionAcquireRequest) (*ports.CompletionAcquireResult, error) {
	panic("unexpected call")
}

func (s *stubInlineSessionRepository) ConfirmCompletionOwner(context.Context, string, string, string) (*domain.Session, error) {
	panic("unexpected call")
}

type stubInlineObjectWriter struct {
	calls       int
	ref         storage.ObjectRef
	contentType string
	size        int64
	body        []byte
	err         error
}

func (s *stubInlineObjectWriter) PutObject(_ context.Context, ref storage.ObjectRef, contentType string, body io.Reader, size int64) error {
	s.calls++
	s.ref = ref
	s.contentType = contentType
	s.size = size
	if body != nil {
		data, _ := io.ReadAll(body)
		s.body = data
	}
	return s.err
}
