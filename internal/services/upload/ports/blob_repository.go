package ports

import "context"

type BlobRecord struct {
	BlobID          string
	TenantID        string
	StorageProvider string
	BucketName      string
	ObjectKey       string
	SizeBytes       int64
	ContentType     string
	ETag            string
	Checksum        string
}

type BlobRepository interface {
	Upsert(ctx context.Context, record BlobRecord) error
	AdjustReferenceCount(ctx context.Context, blobID string, delta int64) error
}
