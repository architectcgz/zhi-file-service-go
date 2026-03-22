package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

func BenchmarkCreateUploadSessionInline(b *testing.B) {
	now := time.Date(2026, 3, 22, 20, 0, 0, 0, time.UTC)
	handler := commands.NewCreateUploadSessionHandler(
		&stubSessionRepository{},
		&stubTenantPolicyReader{policy: ports.TenantUploadPolicy{
			AllowInlineUpload: true,
			AllowMultipart:    true,
			MaxInlineSize:     1024 * 1024,
			MaxFileSize:       4 * 1024 * 1024,
			AllowedMimeTypes:  []string{"application/pdf"},
		}},
		&stubBucketResolver{
			buckets: map[storage.AccessLevel]storage.BucketRef{
				storage.AccessLevelPrivate: {
					Provider:   storage.ProviderS3,
					BucketName: "private-bucket",
				},
			},
		},
		&stubMultipartManager{},
		&stubPresignManager{},
		&stubIDGenerator{id: "bench-upload-session"},
		clock.NewFixed(now),
		commands.CreateUploadSessionConfig{
			SessionTTL:   30 * time.Minute,
			PresignTTL:   10 * time.Minute,
			AllowedModes: []domain.SessionMode{domain.SessionModeInline, domain.SessionModePresignedSingle, domain.SessionModeDirect},
		},
	)

	command := commands.CreateUploadSessionCommand{
		FileName:    "report.pdf",
		ContentType: "application/pdf",
		SizeBytes:   512 * 1024,
		UploadMode:  domain.SessionModeInline,
		Auth:        newUploadAuth(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := handler.Handle(context.Background(), command); err != nil {
			b.Fatalf("Handle() error = %v", err)
		}
	}
}

func BenchmarkCompleteUploadSessionPresignedSingle(b *testing.B) {
	now := time.Date(2026, 3, 22, 20, 5, 0, 0, time.UTC)
	template := mustNewSessionForBenchmark(b, domain.CreateSessionParams{
		ID:          "bench-complete-upload",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   182044,
		AccessLevel: storage.AccessLevelPublic,
		Mode:        domain.SessionModePresignedSingle,
		Object: storage.ObjectRef{
			Provider:   storage.ProviderS3,
			BucketName: "public-bucket",
			ObjectKey:  "tenant-a/uploads/bench-complete-upload/avatar.png",
		},
		Hash:      &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(29 * time.Minute),
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo := &benchmarkCompleteSessionRepository{template: template}
		handler := commands.NewCompleteUploadSessionHandler(
			repo,
			&stubSessionPartRepository{},
			&stubBlobRepository{},
			&stubFileRepository{},
			&stubCompleteDedupRepository{},
			&stubTenantUsageRepository{},
			&stubUploadOutboxPublisher{},
			&stubUploadTxManager{},
			&stubCompleteMultipartManager{},
			&stubObjectReader{
				metadata: storage.ObjectMetadata{
					SizeBytes:   182044,
					ContentType: "image/png",
					ETag:        `"etag-bench"`,
					Checksum:    validHash(),
				},
			},
			&stubSequenceIDGenerator{ids: []string{"blob-bench", "file-bench"}},
			clock.NewFixed(now),
		)

		if _, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
			UploadSessionID: template.ID,
			IdempotencyKey:  "bench-complete",
			Auth:            newUploadAuth(),
		}); err != nil {
			b.Fatalf("Handle() error = %v", err)
		}
	}
}

type benchmarkCompleteSessionRepository struct {
	template *domain.Session
	current  *domain.Session
}

func (s *benchmarkCompleteSessionRepository) Create(context.Context, *domain.Session) error {
	panic("unexpected call")
}

func (s *benchmarkCompleteSessionRepository) Save(_ context.Context, session *domain.Session) error {
	s.current = cloneBenchmarkSession(session)
	return nil
}

func (s *benchmarkCompleteSessionRepository) GetByID(context.Context, string, string) (*domain.Session, error) {
	return cloneBenchmarkSession(s.template), nil
}

func (s *benchmarkCompleteSessionRepository) FindReusable(context.Context, ports.ReusableSessionQuery) (*domain.Session, error) {
	panic("unexpected call")
}

func (s *benchmarkCompleteSessionRepository) AcquireCompletion(_ context.Context, _ ports.CompletionAcquireRequest) (*ports.CompletionAcquireResult, error) {
	s.current = cloneBenchmarkSession(s.template)
	return &ports.CompletionAcquireResult{
		Session:   s.current,
		Ownership: domain.CompletionOwnershipAcquired,
	}, nil
}

func (s *benchmarkCompleteSessionRepository) ConfirmCompletionOwner(context.Context, string, string, string) (*domain.Session, error) {
	return s.current, nil
}

func cloneBenchmarkSession(session *domain.Session) *domain.Session {
	if session == nil {
		return nil
	}
	cloned := *session
	if session.Hash != nil {
		hash := *session.Hash
		cloned.Hash = &hash
	}
	if session.CompletionStartedAt != nil {
		value := *session.CompletionStartedAt
		cloned.CompletionStartedAt = &value
	}
	if session.CompletedAt != nil {
		value := *session.CompletedAt
		cloned.CompletedAt = &value
	}
	if session.AbortedAt != nil {
		value := *session.AbortedAt
		cloned.AbortedAt = &value
	}
	if session.FailedAt != nil {
		value := *session.FailedAt
		cloned.FailedAt = &value
	}
	return &cloned
}

func mustNewSessionForBenchmark(tb testing.TB, params domain.CreateSessionParams) *domain.Session {
	tb.Helper()

	session, err := domain.NewSession(params)
	if err != nil {
		tb.Fatalf("NewSession() error = %v", err)
	}
	return session
}
