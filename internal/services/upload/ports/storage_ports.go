package ports

import (
	"context"
	"io"
	"time"

	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type BucketResolver interface {
	Resolve(accessLevel pkgstorage.AccessLevel) (pkgstorage.BucketRef, error)
	Normalize(bucketName string) string
}

type ObjectReader interface {
	HeadObject(ctx context.Context, ref pkgstorage.ObjectRef) (pkgstorage.ObjectMetadata, error)
}

type InlineObjectWriter interface {
	PutObject(ctx context.Context, ref pkgstorage.ObjectRef, contentType string, body io.Reader, size int64) error
}

type MultipartManager interface {
	CreateMultipartUpload(ctx context.Context, ref pkgstorage.ObjectRef, contentType string) (string, error)
	UploadPart(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string, partNumber int, body io.Reader, size int64) (string, error)
	ListUploadedParts(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string) ([]pkgstorage.UploadedPart, error)
	CompleteMultipartUpload(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string, parts []pkgstorage.UploadedPart) error
	AbortMultipartUpload(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string) error
}

type PresignManager interface {
	PresignPutObject(ctx context.Context, ref pkgstorage.ObjectRef, contentType string, ttl time.Duration) (string, map[string]string, error)
	PresignUploadPart(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string, partNumber int, ttl time.Duration) (string, map[string]string, error)
}
