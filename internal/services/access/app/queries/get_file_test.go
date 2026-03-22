package queries_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestGetFileReturnsPublicDownloadURLWhenEnabled(t *testing.T) {
	repo := &stubFileReadRepository{
		file: domain.FileView{
			FileID:          "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
			TenantID:        "tenant-a",
			FileName:        "avatar.png",
			AccessLevel:     storage.AccessLevelPublic,
			Status:          domain.FileStatusActive,
			StorageProvider: storage.ProviderS3,
			BucketName:      "public",
			ObjectKey:       "tenant-a/avatar.png",
			CreatedAt:       time.Date(2026, 3, 22, 8, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2026, 3, 22, 8, 0, 1, 0, time.UTC),
		},
	}
	locator := &stubObjectLocator{url: "https://cdn.example.com/public/tenant-a/avatar.png"}
	handler := queries.NewGetFileHandler(repo, locator, true)

	result, err := handler.Handle(context.Background(), queries.GetFileQuery{
		FileID: "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  "tenant-a",
			Scopes:    []string{domain.ScopeFileRead},
		},
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.DownloadURL != locator.url {
		t.Fatalf("expected download url %q, got %q", locator.url, result.DownloadURL)
	}
}

func TestGetFileRejectsMissingReadScope(t *testing.T) {
	handler := queries.NewGetFileHandler(&stubFileReadRepository{}, &stubObjectLocator{}, true)

	_, err := handler.Handle(context.Background(), queries.GetFileQuery{
		FileID: "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  "tenant-a",
		},
	})
	if xerrors.CodeOf(err) != xerrors.CodeForbidden {
		t.Fatalf("expected forbidden error, got %s (err=%v)", xerrors.CodeOf(err), err)
	}
}

type stubFileReadRepository struct {
	file domain.FileView
	err  error
}

func (s *stubFileReadRepository) GetByID(_ context.Context, fileID string) (domain.FileView, error) {
	if s.err != nil {
		return domain.FileView{}, s.err
	}
	if s.file.FileID == "" || s.file.FileID != fileID {
		return domain.FileView{}, domain.ErrFileNotFound(fileID)
	}
	return s.file, nil
}

type stubObjectLocator struct {
	calls int
	url   string
	err   error
}

func (s *stubObjectLocator) ResolveObjectURL(ref storage.ObjectRef) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	if ref.ObjectKey == "" {
		return "", errors.New("missing object key")
	}
	return s.url, nil
}
