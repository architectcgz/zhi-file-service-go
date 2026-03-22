package commands_test

import (
	"context"
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

func TestCreateUploadSessionInlineCreatesDefaultPrivateSession(t *testing.T) {
	now := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	sessions := &stubSessionRepository{}
	buckets := &stubBucketResolver{
		buckets: map[storage.AccessLevel]storage.BucketRef{
			storage.AccessLevelPrivate: {
				Provider:   storage.ProviderS3,
				BucketName: "private-bucket",
			},
		},
	}
	handler := commands.NewCreateUploadSessionHandler(
		sessions,
		&stubTenantPolicyReader{policy: ports.TenantUploadPolicy{
			AllowInlineUpload: true,
			AllowMultipart:    true,
			MaxInlineSize:     1024 * 1024,
			MaxFileSize:       2 * 1024 * 1024,
			AllowedMimeTypes:  []string{"application/pdf"},
		}},
		buckets,
		&stubMultipartManager{},
		&stubPresignManager{},
		&stubIDGenerator{id: "01JQ31M2G2Z9MWX5B5N4VQX7J1"},
		clock.NewFixed(now),
		commands.CreateUploadSessionConfig{
			SessionTTL:   30 * time.Minute,
			PresignTTL:   10 * time.Minute,
			AllowedModes: []domain.SessionMode{domain.SessionModeInline, domain.SessionModePresignedSingle, domain.SessionModeDirect},
		},
	)

	result, err := handler.Handle(context.Background(), commands.CreateUploadSessionCommand{
		FileName:    "report 2026.pdf",
		ContentType: "application/pdf",
		SizeBytes:   512 * 1024,
		UploadMode:  domain.SessionModeInline,
		Auth:        newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.UploadSessionID != "01JQ31M2G2Z9MWX5B5N4VQX7J1" {
		t.Fatalf("unexpected upload session id: %s", result.UploadSessionID)
	}
	if result.AccessLevel != storage.AccessLevelPrivate {
		t.Fatalf("expected default private access level, got %s", result.AccessLevel)
	}
	if result.PutURL != "" {
		t.Fatalf("expected no put url for inline upload, got %s", result.PutURL)
	}
	if sessions.created == nil {
		t.Fatal("expected session to be created")
	}
	if sessions.created.Object.BucketName != "private-bucket" {
		t.Fatalf("unexpected bucket name: %s", sessions.created.Object.BucketName)
	}
	if sessions.created.Object.ObjectKey != "tenant-a/uploads/01JQ31M2G2Z9MWX5B5N4VQX7J1/report_2026.pdf" {
		t.Fatalf("unexpected object key: %s", sessions.created.Object.ObjectKey)
	}
	if sessions.created.ExpiresAt != now.Add(30*time.Minute) {
		t.Fatalf("unexpected expires at: %s", sessions.created.ExpiresAt)
	}
	if len(buckets.levels) != 1 || buckets.levels[0] != storage.AccessLevelPrivate {
		t.Fatalf("unexpected bucket resolution levels: %#v", buckets.levels)
	}
}

func TestCreateUploadSessionPresignedSingleReturnsPutURL(t *testing.T) {
	now := time.Date(2026, 3, 22, 10, 10, 0, 0, time.UTC)
	sessions := &stubSessionRepository{}
	presign := &stubPresignManager{
		url: "https://storage.example.com/upload",
		headers: map[string]string{
			"Content-Type": "image/png",
		},
	}
	handler := commands.NewCreateUploadSessionHandler(
		sessions,
		&stubTenantPolicyReader{policy: ports.TenantUploadPolicy{
			AllowInlineUpload: true,
			AllowMultipart:    true,
			MaxFileSize:       4 * 1024 * 1024,
			AllowedMimeTypes:  []string{"image/png"},
		}},
		&stubBucketResolver{
			buckets: map[storage.AccessLevel]storage.BucketRef{
				storage.AccessLevelPublic: {
					Provider:   storage.ProviderS3,
					BucketName: "public-bucket",
				},
			},
		},
		&stubMultipartManager{},
		presign,
		&stubIDGenerator{id: "01JQ31P1SV4DFM4KJ2TQSPG6R5"},
		clock.NewFixed(now),
		commands.CreateUploadSessionConfig{
			SessionTTL:   45 * time.Minute,
			PresignTTL:   12 * time.Minute,
			AllowedModes: []domain.SessionMode{domain.SessionModeInline, domain.SessionModePresignedSingle, domain.SessionModeDirect},
		},
	)

	result, err := handler.Handle(context.Background(), commands.CreateUploadSessionCommand{
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   182044,
		AccessLevel: storage.AccessLevelPublic,
		UploadMode:  domain.SessionModePresignedSingle,
		ContentHash: &domain.ContentHash{
			Algorithm: "sha256",
			Value:     validHash(),
		},
		Auth: newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.PutURL != presign.url {
		t.Fatalf("expected put url %q, got %q", presign.url, result.PutURL)
	}
	if presign.calls != 1 {
		t.Fatalf("expected one presign call, got %d", presign.calls)
	}
	if presign.lastTTL != 12*time.Minute {
		t.Fatalf("unexpected presign ttl: %s", presign.lastTTL)
	}
	if sessions.created == nil {
		t.Fatal("expected session to be created")
	}
	if sessions.created.Hash == nil || sessions.created.Hash.Algorithm != "SHA256" {
		t.Fatalf("expected normalized hash, got %#v", sessions.created.Hash)
	}
	if sessions.created.ProviderUploadID != "" {
		t.Fatalf("expected empty provider upload id, got %q", sessions.created.ProviderUploadID)
	}
}

func TestCreateUploadSessionDirectCreatesMultipartUpload(t *testing.T) {
	now := time.Date(2026, 3, 22, 10, 20, 0, 0, time.UTC)
	sessions := &stubSessionRepository{}
	multipart := &stubMultipartManager{uploadID: "upload-123"}
	handler := commands.NewCreateUploadSessionHandler(
		sessions,
		&stubTenantPolicyReader{policy: ports.TenantUploadPolicy{
			AllowInlineUpload: true,
			AllowMultipart:    true,
			MaxFileSize:       1024 * 1024 * 1024,
			AllowedMimeTypes:  []string{"video/mp4"},
		}},
		&stubBucketResolver{
			buckets: map[storage.AccessLevel]storage.BucketRef{
				storage.AccessLevelPrivate: {
					Provider:   storage.ProviderMinIO,
					BucketName: "private-video",
				},
			},
		},
		multipart,
		&stubPresignManager{},
		&stubIDGenerator{id: "01JQ31RVX0QPCNAE5FBCMA69JF"},
		clock.NewFixed(now),
		commands.CreateUploadSessionConfig{
			SessionTTL:   30 * time.Minute,
			PresignTTL:   15 * time.Minute,
			AllowedModes: []domain.SessionMode{domain.SessionModeInline, domain.SessionModePresignedSingle, domain.SessionModeDirect},
		},
	)

	result, err := handler.Handle(context.Background(), commands.CreateUploadSessionCommand{
		FileName:    "promo.mp4",
		ContentType: "video/mp4",
		SizeBytes:   734003200,
		UploadMode:  domain.SessionModeDirect,
		TotalParts:  8,
		ContentHash: &domain.ContentHash{
			Algorithm: "SHA256",
			Value:     validHash(),
		},
		Auth: newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.TotalParts != 8 {
		t.Fatalf("expected total parts 8, got %d", result.TotalParts)
	}
	if sessions.created == nil {
		t.Fatal("expected session to be created")
	}
	if sessions.created.ProviderUploadID != "upload-123" {
		t.Fatalf("unexpected provider upload id: %s", sessions.created.ProviderUploadID)
	}
	if multipart.createCalls != 1 {
		t.Fatalf("expected one multipart creation, got %d", multipart.createCalls)
	}
	if multipart.lastRef.ObjectKey != sessions.created.Object.ObjectKey {
		t.Fatalf("expected multipart ref %q, got %q", sessions.created.Object.ObjectKey, multipart.lastRef.ObjectKey)
	}
}

func TestCreateUploadSessionRejectsPolicyViolations(t *testing.T) {
	tests := []struct {
		name    string
		command commands.CreateUploadSessionCommand
		policy  ports.TenantUploadPolicy
		want    xerrors.Code
	}{
		{
			name: "inline disabled",
			command: commands.CreateUploadSessionCommand{
				FileName:    "report.pdf",
				ContentType: "application/pdf",
				SizeBytes:   1024,
				UploadMode:  domain.SessionModeInline,
				Auth:        newUploadAuth(),
			},
			policy: ports.TenantUploadPolicy{
				AllowMultipart:   true,
				AllowedMimeTypes: []string{"application/pdf"},
			},
			want: domain.CodeUploadModeInvalid,
		},
		{
			name: "mime type denied",
			command: commands.CreateUploadSessionCommand{
				FileName:    "avatar.gif",
				ContentType: "image/gif",
				SizeBytes:   1024,
				UploadMode:  domain.SessionModeInline,
				Auth:        newUploadAuth(),
			},
			policy: ports.TenantUploadPolicy{
				AllowInlineUpload: true,
				AllowMultipart:    true,
				AllowedMimeTypes:  []string{"image/png"},
			},
			want: domain.CodeMimeTypeNotAllowed,
		},
		{
			name: "quota exceeded",
			command: commands.CreateUploadSessionCommand{
				FileName:    "movie.mp4",
				ContentType: "video/mp4",
				SizeBytes:   4096,
				UploadMode:  domain.SessionModeDirect,
				TotalParts:  2,
				ContentHash: &domain.ContentHash{
					Algorithm: "SHA256",
					Value:     validHash(),
				},
				Auth: newUploadAuth(),
			},
			policy: ports.TenantUploadPolicy{
				AllowMultipart:   true,
				MaxFileSize:      2048,
				AllowedMimeTypes: []string{"video/mp4"},
			},
			want: domain.CodeTenantQuotaExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := commands.NewCreateUploadSessionHandler(
				&stubSessionRepository{},
				&stubTenantPolicyReader{policy: tt.policy},
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
				&stubIDGenerator{id: "01JQ31WKK55QGNRVG764BHJ0CA"},
				clock.NewFixed(time.Date(2026, 3, 22, 10, 30, 0, 0, time.UTC)),
				commands.CreateUploadSessionConfig{
					AllowedModes: []domain.SessionMode{domain.SessionModeInline, domain.SessionModePresignedSingle, domain.SessionModeDirect},
				},
			)

			_, err := handler.Handle(context.Background(), tt.command)
			if code := xerrors.CodeOf(err); code != tt.want {
				t.Fatalf("expected error code %s, got %s (err=%v)", tt.want, code, err)
			}
		})
	}
}

func TestCreateUploadSessionReturnsReusableSession(t *testing.T) {
	now := time.Date(2026, 3, 22, 10, 40, 0, 0, time.UTC)
	reusable := mustNewSession(t, domain.CreateSessionParams{
		ID:             "01JQ31Z0BY6QVB4ZXNS0J2MAFA",
		TenantID:       "tenant-a",
		OwnerID:        "user-1",
		FileName:       "avatar.png",
		ContentType:    "image/png",
		SizeBytes:      182044,
		AccessLevel:    storage.AccessLevelPublic,
		Mode:           domain.SessionModePresignedSingle,
		Object:         storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "public-bucket", ObjectKey: "tenant-a/uploads/01JQ31Z0BY6QVB4ZXNS0J2MAFA/avatar.png"},
		Hash:           &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt:      now.Add(-5 * time.Minute),
		UpdatedAt:      now.Add(-5 * time.Minute),
		IdempotencyKey: "idem-1",
		ExpiresAt:      now.Add(25 * time.Minute),
	})
	sessions := &stubSessionRepository{reusable: reusable}
	presign := &stubPresignManager{
		url: "https://storage.example.com/reused",
		headers: map[string]string{
			"Content-Type": "image/png",
		},
	}
	idgen := &stubIDGenerator{id: "01JQ3201N8X6GEBEPNE3QZPS0P"}
	handler := commands.NewCreateUploadSessionHandler(
		sessions,
		&stubTenantPolicyReader{policy: ports.TenantUploadPolicy{
			AllowInlineUpload: true,
			AllowMultipart:    true,
			MaxFileSize:       4 * 1024 * 1024,
			AllowedMimeTypes:  []string{"image/png"},
		}},
		&stubBucketResolver{},
		&stubMultipartManager{},
		presign,
		idgen,
		clock.NewFixed(now),
		commands.CreateUploadSessionConfig{
			AllowedModes: []domain.SessionMode{domain.SessionModeInline, domain.SessionModePresignedSingle, domain.SessionModeDirect},
		},
	)

	result, err := handler.Handle(context.Background(), commands.CreateUploadSessionCommand{
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   182044,
		AccessLevel: storage.AccessLevelPublic,
		UploadMode:  domain.SessionModePresignedSingle,
		ContentHash: &domain.ContentHash{
			Algorithm: "SHA256",
			Value:     validHash(),
		},
		Auth: newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.UploadSessionID != reusable.ID {
		t.Fatalf("expected reused session id %q, got %q", reusable.ID, result.UploadSessionID)
	}
	if result.PutURL != presign.url {
		t.Fatalf("expected presigned put url %q, got %q", presign.url, result.PutURL)
	}
	if sessions.created != nil {
		t.Fatal("expected no new session creation for reusable session")
	}
	if idgen.calls != 0 {
		t.Fatalf("expected id generator to be skipped, got %d calls", idgen.calls)
	}
}

type stubSessionRepository struct {
	created       *domain.Session
	createErr     error
	reusable      *domain.Session
	reusableErr   error
	reusableQuery ports.ReusableSessionQuery
}

func (s *stubSessionRepository) Create(_ context.Context, session *domain.Session) error {
	s.created = session
	return s.createErr
}

func (s *stubSessionRepository) GetByID(context.Context, string, string) (*domain.Session, error) {
	panic("unexpected call")
}

func (s *stubSessionRepository) FindReusable(_ context.Context, query ports.ReusableSessionQuery) (*domain.Session, error) {
	s.reusableQuery = query
	return s.reusable, s.reusableErr
}

func (s *stubSessionRepository) AcquireCompletion(context.Context, ports.CompletionAcquireRequest) (*ports.CompletionAcquireResult, error) {
	panic("unexpected call")
}

func (s *stubSessionRepository) ConfirmCompletionOwner(context.Context, string, string, string) (*domain.Session, error) {
	panic("unexpected call")
}

type stubTenantPolicyReader struct {
	policy ports.TenantUploadPolicy
	err    error
}

func (s *stubTenantPolicyReader) ReadUploadPolicy(context.Context, string) (ports.TenantUploadPolicy, error) {
	return s.policy, s.err
}

type stubBucketResolver struct {
	buckets map[storage.AccessLevel]storage.BucketRef
	levels  []storage.AccessLevel
}

func (s *stubBucketResolver) Resolve(accessLevel storage.AccessLevel) (storage.BucketRef, error) {
	s.levels = append(s.levels, accessLevel)
	return s.buckets[accessLevel], nil
}

func (s *stubBucketResolver) Normalize(bucketName string) string {
	return bucketName
}

type stubMultipartManager struct {
	uploadID    string
	createCalls int
	lastRef     storage.ObjectRef
}

func (s *stubMultipartManager) CreateMultipartUpload(_ context.Context, ref storage.ObjectRef, _ string) (string, error) {
	s.createCalls++
	s.lastRef = ref
	return s.uploadID, nil
}

func (s *stubMultipartManager) UploadPart(context.Context, storage.ObjectRef, string, int, io.Reader, int64) (string, error) {
	panic("unexpected call")
}

func (s *stubMultipartManager) ListUploadedParts(context.Context, storage.ObjectRef, string) ([]storage.UploadedPart, error) {
	panic("unexpected call")
}

func (s *stubMultipartManager) CompleteMultipartUpload(context.Context, storage.ObjectRef, string, []storage.UploadedPart) error {
	panic("unexpected call")
}

func (s *stubMultipartManager) AbortMultipartUpload(context.Context, storage.ObjectRef, string) error {
	panic("unexpected call")
}

type stubPresignManager struct {
	url      string
	headers  map[string]string
	err      error
	calls    int
	lastRef  storage.ObjectRef
	lastType string
	lastTTL  time.Duration
}

func (s *stubPresignManager) PresignPutObject(_ context.Context, ref storage.ObjectRef, contentType string, ttl time.Duration) (string, map[string]string, error) {
	s.calls++
	s.lastRef = ref
	s.lastType = contentType
	s.lastTTL = ttl
	return s.url, s.headers, s.err
}

func (s *stubPresignManager) PresignUploadPart(context.Context, storage.ObjectRef, string, int, time.Duration) (string, map[string]string, error) {
	panic("unexpected call")
}

type stubIDGenerator struct {
	id    string
	calls int
}

func (s *stubIDGenerator) New() (string, error) {
	s.calls++
	return s.id, nil
}

func newUploadAuth() domain.AuthContext {
	return domain.AuthContext{
		SubjectID: "user-1",
		TenantID:  "tenant-a",
		Scopes:    []string{domain.ScopeFileWrite},
	}
}

func validHash() string {
	return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}

func mustNewSession(t *testing.T, params domain.CreateSessionParams) *domain.Session {
	t.Helper()

	session, err := domain.NewSession(params)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	return session
}
