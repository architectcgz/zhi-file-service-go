package ports

import "context"

type FileAssetRecord struct {
	FileID          string
	TenantID        string
	BlobID          string
	FileName        string
	ContentType     string
	SizeBytes       int64
	AccessLevel     string
	StorageProvider string
	BucketName      string
	ObjectKey       string
}

type FileRepository interface {
	CreateFileAsset(ctx context.Context, record FileAssetRecord) error
}
