package ports

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type BlobRecord struct {
	BlobID          string
	TenantID        string
	StorageProvider pkgstorage.Provider
	BucketName      string
	ObjectKey       string
	SizeBytes       int64
	ContentType     string
	ETag            string
	Checksum        string
	Hash            domain.ContentHash
}

type BlobRepository interface {
	Upsert(ctx context.Context, record BlobRecord) error
	AdjustReferenceCount(ctx context.Context, blobID string, delta int64) error
}
