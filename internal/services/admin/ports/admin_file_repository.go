package ports

import (
	"context"
	"time"

	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type AdminFileView struct {
	FileID      string
	TenantID    string
	OwnerID     string
	BlobID      string
	FileName    string
	ContentType string
	SizeBytes   int64
	AccessLevel pkgstorage.AccessLevel
	Status      string
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ListFilesQuery struct {
	TenantID     string
	TenantScopes []string
	Status       string
	Cursor       string
	Limit        int
}

type DeleteFileRecord struct {
	File                    AdminFileView
	PhysicalDeleteScheduled bool
	AlreadyDeleted          bool
}

type AdminFileRepository interface {
	GetByID(ctx context.Context, fileID string) (*AdminFileView, error)
	List(ctx context.Context, query ListFilesQuery) ([]AdminFileView, string, error)
	MarkDeleted(ctx context.Context, fileID string, deletedAt time.Time) (*DeleteFileRecord, error)
}
