package queries_test

import (
	"context"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

func BenchmarkGetFilePublic(b *testing.B) {
	file := newDownloadTestFile(storage.AccessLevelPublic)
	repo := &stubFileReadRepository{file: file}
	locator := &stubObjectLocator{url: "https://cdn.example.com/public/object"}
	handler := queries.NewGetFileHandler(repo, locator, true)
	query := queries.GetFileQuery{
		FileID: file.FileID,
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  file.TenantID,
			Scopes:    []string{domain.ScopeFileRead},
		},
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := handler.Handle(ctx, query); err != nil {
			b.Fatalf("Handle() error = %v", err)
		}
	}
}

func BenchmarkResolveDownloadPrivate(b *testing.B) {
	file := newDownloadTestFile(storage.AccessLevelPrivate)
	repo := &stubFileReadRepository{file: file}
	policies := &stubTenantPolicyReader{policy: domain.TenantPolicy{TenantID: file.TenantID}}
	presign := &stubPresignManager{url: "https://s3.example.com/private/object?signature=1"}
	handler := queries.NewResolveDownloadHandler(repo, policies, &stubObjectLocator{}, presign, 2*time.Minute, true)
	query := queries.ResolveDownloadQuery{
		FileID:      file.FileID,
		Disposition: domain.DownloadDispositionAttachment,
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  file.TenantID,
			Scopes:    []string{domain.ScopeFileRead},
		},
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := handler.Handle(ctx, query); err != nil {
			b.Fatalf("Handle() error = %v", err)
		}
	}
}

func BenchmarkRedirectByAccessTicketPrivate(b *testing.B) {
	now := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
	file := newDownloadTestFile(storage.AccessLevelPrivate)
	repo := &stubFileReadRepository{file: file}
	policies := &stubTenantPolicyReader{policy: domain.TenantPolicy{TenantID: file.TenantID}}
	issuer := &stubRedirectTicketIssuer{
		claims: domain.AccessTicketClaims{
			FileID:      file.FileID,
			TenantID:    file.TenantID,
			Subject:     "user-1",
			SubjectType: "USER",
			Disposition: domain.DownloadDispositionAttachment,
			ExpiresAt:   now.Add(2 * time.Minute),
		},
	}
	presign := &stubPresignManager{url: "https://s3.example.com/private/object?signature=1"}
	handler := queries.NewRedirectByAccessTicketHandler(
		repo,
		policies,
		issuer,
		&stubObjectLocator{},
		presign,
		clock.NewFixed(now),
		2*time.Minute,
		true,
	)

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := handler.Handle(ctx, queries.RedirectByAccessTicketQuery{
			Ticket: "at_01JQ2QFJ7X0C24C25J24E2RYN9",
		}); err != nil {
			b.Fatalf("Handle() error = %v", err)
		}
	}
}
