package ports

import (
	"context"
	"time"
)

type AdminFileView struct {
	FileID      string
	TenantID    string
	OwnerID     string
	FileName    string
	Status      string
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ListFilesQuery struct {
	TenantID string
	Status   string
	Cursor   string
	Limit    int
}

type AdminFileRepository interface {
	GetByID(ctx context.Context, fileID string) (*AdminFileView, error)
	List(ctx context.Context, query ListFilesQuery) ([]AdminFileView, string, error)
	MarkDeleted(ctx context.Context, fileID string, deletedAt time.Time) (*AdminFileView, error)
}
