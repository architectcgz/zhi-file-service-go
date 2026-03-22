package queries_test

import (
	"context"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestResolveDownloadUsesPublicURLOrPresignByAccessLevel(t *testing.T) {
	tests := []struct {
		name             string
		file             domain.FileView
		publicURLEnabled bool
		wantURL          string
		wantLocatorCalls int
		wantPresignCalls int
	}{
		{
			name:             "public file prefers public url",
			file:             newDownloadTestFile(storage.AccessLevelPublic),
			publicURLEnabled: true,
			wantURL:          "https://cdn.example.com/public/object",
			wantLocatorCalls: 1,
			wantPresignCalls: 0,
		},
		{
			name:             "private file uses presign",
			file:             newDownloadTestFile(storage.AccessLevelPrivate),
			publicURLEnabled: true,
			wantURL:          "https://s3.example.com/private/object?signature=1",
			wantLocatorCalls: 0,
			wantPresignCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubFileReadRepository{file: tt.file}
			policies := &stubTenantPolicyReader{policy: domain.TenantPolicy{TenantID: tt.file.TenantID}}
			locator := &stubObjectLocator{url: "https://cdn.example.com/public/object"}
			presign := &stubPresignManager{url: "https://s3.example.com/private/object?signature=1"}
			handler := queries.NewResolveDownloadHandler(repo, policies, locator, presign, 2*time.Minute, tt.publicURLEnabled)

			result, err := handler.Handle(context.Background(), queries.ResolveDownloadQuery{
				FileID: tt.file.FileID,
				Auth: domain.AuthContext{
					SubjectID: "user-1",
					TenantID:  tt.file.TenantID,
					Scopes:    []string{domain.ScopeFileRead},
				},
			})
			if err != nil {
				t.Fatalf("Handle returned error: %v", err)
			}
			if result.URL != tt.wantURL {
				t.Fatalf("expected url %q, got %q", tt.wantURL, result.URL)
			}
			if locator.calls != tt.wantLocatorCalls {
				t.Fatalf("expected locator calls %d, got %d", tt.wantLocatorCalls, locator.calls)
			}
			if presign.calls != tt.wantPresignCalls {
				t.Fatalf("expected presign calls %d, got %d", tt.wantPresignCalls, presign.calls)
			}
		})
	}
}

func TestResolveDownloadRejectsInlinePreviewWhenPolicyDisallowsIt(t *testing.T) {
	repo := &stubFileReadRepository{file: newDownloadTestFile(storage.AccessLevelPrivate)}
	policies := &stubTenantPolicyReader{policy: domain.TenantPolicy{
		TenantID:              "tenant-a",
		InlinePreviewDisabled: true,
	}}
	handler := queries.NewResolveDownloadHandler(repo, policies, &stubObjectLocator{}, &stubPresignManager{}, 2*time.Minute, true)

	_, err := handler.Handle(context.Background(), queries.ResolveDownloadQuery{
		FileID:      repo.file.FileID,
		Disposition: domain.DownloadDispositionInline,
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  repo.file.TenantID,
			Scopes:    []string{domain.ScopeFileRead},
		},
	})
	if code := xerrors.CodeOf(err); code != domain.CodeDownloadNotAllowed {
		t.Fatalf("expected download not allowed, got %s (err=%v)", code, err)
	}
}

func newDownloadTestFile(level storage.AccessLevel) domain.FileView {
	return domain.FileView{
		FileID:          "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
		TenantID:        "tenant-a",
		FileName:        "invoice.pdf",
		AccessLevel:     level,
		Status:          domain.FileStatusActive,
		StorageProvider: storage.ProviderS3,
		BucketName:      "bucket",
		ObjectKey:       "tenant-a/invoice.pdf",
		CreatedAt:       time.Date(2026, 3, 22, 8, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 3, 22, 8, 0, 1, 0, time.UTC),
	}
}

type stubPresignManager struct {
	url   string
	calls int
}

func (s *stubPresignManager) PresignPutObject(context.Context, storage.ObjectRef, string, time.Duration) (string, map[string]string, error) {
	panic("unexpected call")
}

func (s *stubPresignManager) PresignUploadPart(context.Context, storage.ObjectRef, string, int, time.Duration) (string, map[string]string, error) {
	panic("unexpected call")
}

func (s *stubPresignManager) PresignGetObject(_ context.Context, _ storage.ObjectRef, _ time.Duration) (string, error) {
	s.calls++
	return s.url, nil
}

type stubTenantPolicyReader struct {
	policy domain.TenantPolicy
	err    error
}

func (s *stubTenantPolicyReader) GetByTenantID(context.Context, string) (domain.TenantPolicy, error) {
	return s.policy, s.err
}
