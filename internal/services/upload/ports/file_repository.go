package ports

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type FileAssetRecord struct {
	FileID          string
	TenantID        string
	OwnerID         string
	BlobID          string
	FileName        string
	ContentType     string
	SizeBytes       int64
	AccessLevel     pkgstorage.AccessLevel
	StorageProvider pkgstorage.Provider
	BucketName      string
	ObjectKey       string
	Hash            *domain.ContentHash
}

type FileRepository interface {
	CreateFileAsset(ctx context.Context, record FileAssetRecord) error
}
