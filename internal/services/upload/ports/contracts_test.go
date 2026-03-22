package ports

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

func TestContractTypesExposePhaseOneFacts(t *testing.T) {
	hash := domain.ContentHash{Algorithm: "SHA256", Value: "4f6f0d53c1efb6bb7c9f6b4e5b7d7e2b7b5b2f4b33f3ef0d4ec2ef9f74de4f75"}

	_ = BlobRecord{
		BlobID:          "blob-1",
		TenantID:        "tenant-1",
		StorageProvider: pkgstorage.ProviderS3,
		BucketName:      "bucket-a",
		ObjectKey:       "tenant-1/object",
		Hash:            hash,
	}
	_ = FileAssetRecord{
		FileID:          "file-1",
		TenantID:        "tenant-1",
		OwnerID:         "user-1",
		BlobID:          "blob-1",
		AccessLevel:     pkgstorage.AccessLevelPrivate,
		StorageProvider: pkgstorage.ProviderS3,
		Hash:            &hash,
	}
	_ = ReusableSessionQuery{
		TenantID:    "tenant-1",
		OwnerID:     "user-1",
		Mode:        domain.SessionModeDirect,
		AccessLevel: pkgstorage.AccessLevelPrivate,
		SizeBytes:   1024,
		Hash:        &hash,
	}
	_ = CompletionAcquireRequest{
		TenantID:        "tenant-1",
		UploadSessionID: "upload-session-1",
		CompletionToken: "token-1",
		StartedAt:       time.Now(),
	}
	_ = SessionPartRecord{
		UploadSessionID: "upload-session-1",
		PartNumber:      1,
		ETag:            "etag-1",
		PartSize:        512,
		UploadedAt:      time.Now(),
	}
	_ = DedupLookupKey{
		TenantID:   "tenant-1",
		BucketName: "bucket-a",
		Hash:       hash,
	}
	_ = DedupClaim{
		TenantID:        "tenant-1",
		BucketName:      "bucket-a",
		UploadSessionID: "upload-session-1",
		Hash:            hash,
		OwnerToken:      "owner-token-1",
		ExpiresAt:       time.Now().Add(time.Minute),
	}
}

func TestContractInterfacesReservePhaseTwoSemantics(t *testing.T) {
	var sessionRepo SessionRepository = stubSessionRepository{}
	var partRepo SessionPartRepository = stubSessionPartRepository{}
	var dedupRepo DedupRepository = stubDedupRepository{}
	var multipart MultipartManager = stubMultipartManager{}
	var presign PresignManager = stubPresignManager{}

	if sessionRepo == nil || partRepo == nil || dedupRepo == nil || multipart == nil || presign == nil {
		t.Fatal("expected non-nil interfaces")
	}
}

type stubSessionRepository struct{}

func (stubSessionRepository) Create(context.Context, *domain.Session) error {
	return nil
}

func (stubSessionRepository) Save(context.Context, *domain.Session) error {
	return nil
}

func (stubSessionRepository) GetByID(context.Context, string, string) (*domain.Session, error) {
	return nil, nil
}

func (stubSessionRepository) FindReusable(context.Context, ReusableSessionQuery) (*domain.Session, error) {
	return nil, nil
}

func (stubSessionRepository) AcquireCompletion(context.Context, CompletionAcquireRequest) (*CompletionAcquireResult, error) {
	return nil, nil
}

func (stubSessionRepository) ConfirmCompletionOwner(context.Context, string, string, string) (*domain.Session, error) {
	return nil, nil
}

type stubSessionPartRepository struct{}

func (stubSessionPartRepository) ListBySessionID(context.Context, string, string) ([]SessionPartRecord, error) {
	return nil, nil
}

func (stubSessionPartRepository) Upsert(context.Context, SessionPartRecord) error {
	return nil
}

func (stubSessionPartRepository) Replace(context.Context, string, string, []SessionPartRecord) error {
	return nil
}

type stubDedupRepository struct{}

func (stubDedupRepository) LookupByHash(context.Context, DedupLookupKey) (*domain.DedupDecision, error) {
	return nil, nil
}

func (stubDedupRepository) ClaimHash(context.Context, DedupClaim) error {
	return nil
}

type stubMultipartManager struct{}

func (stubMultipartManager) CreateMultipartUpload(context.Context, pkgstorage.ObjectRef, string) (string, error) {
	return "", nil
}

func (stubMultipartManager) UploadPart(context.Context, pkgstorage.ObjectRef, string, int, io.Reader, int64) (string, error) {
	return "", nil
}

func (stubMultipartManager) ListUploadedParts(context.Context, pkgstorage.ObjectRef, string) ([]pkgstorage.UploadedPart, error) {
	return nil, nil
}

func (stubMultipartManager) CompleteMultipartUpload(context.Context, pkgstorage.ObjectRef, string, []pkgstorage.UploadedPart) error {
	return nil
}

func (stubMultipartManager) AbortMultipartUpload(context.Context, pkgstorage.ObjectRef, string) error {
	return nil
}

type stubPresignManager struct{}

func (stubPresignManager) PresignPutObject(context.Context, pkgstorage.ObjectRef, string, time.Duration) (string, map[string]string, error) {
	return "", nil, nil
}

func (stubPresignManager) PresignUploadPart(context.Context, pkgstorage.ObjectRef, string, int, time.Duration) (string, map[string]string, error) {
	return "", nil, nil
}
