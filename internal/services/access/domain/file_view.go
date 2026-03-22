package domain

import (
	"time"

	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type FileStatus string

const (
	FileStatusActive  FileStatus = "ACTIVE"
	FileStatusDeleted FileStatus = "DELETED"
)

type FileView struct {
	FileID          string
	TenantID        string
	OwnerID         string
	FileName        string
	ContentType     string
	SizeBytes       int64
	AccessLevel     storage.AccessLevel
	Status          FileStatus
	StorageProvider storage.Provider
	BucketName      string
	ObjectKey       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (f FileView) EnsureReadable(auth AuthContext) error {
	if f.Status != FileStatusActive {
		return ErrFileAccessDenied(f.FileID)
	}
	if auth.TenantID != f.TenantID {
		return ErrTenantScopeDenied(f.FileID)
	}

	return nil
}

func (f FileView) ObjectRef() storage.ObjectRef {
	return storage.ObjectRef{
		Provider:   f.StorageProvider,
		BucketName: f.BucketName,
		ObjectKey:  f.ObjectKey,
	}
}
