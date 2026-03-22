package view

import (
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type AdminFile struct {
	FileID      string
	TenantID    string
	FileName    string
	ContentType string
	SizeBytes   int64
	AccessLevel pkgstorage.AccessLevel
	Status      string
	DeletedAt   *time.Time
	CreatedAt   Time
	UpdatedAt   Time
}

type AdminFileList struct {
	Items      []AdminFile
	NextCursor string
}

type DeleteFileResult struct {
	FileID                  string
	Status                  string
	DeletedAt               *time.Time
	PhysicalDeleteScheduled bool
}

func FromAdminFile(value *ports.AdminFileView) AdminFile {
	if value == nil {
		return AdminFile{}
	}

	return AdminFile{
		FileID:      value.FileID,
		TenantID:    value.TenantID,
		FileName:    value.FileName,
		ContentType: value.ContentType,
		SizeBytes:   value.SizeBytes,
		AccessLevel: value.AccessLevel,
		Status:      value.Status,
		DeletedAt:   cloneTimePtr(value.DeletedAt),
		CreatedAt:   Time(value.CreatedAt),
		UpdatedAt:   Time(value.UpdatedAt),
	}
}

func FromAdminFiles(values []ports.AdminFileView) []AdminFile {
	items := make([]AdminFile, 0, len(values))
	for i := range values {
		items = append(items, FromAdminFile(&values[i]))
	}

	return items
}

func FromDeleteFileRecord(value *ports.DeleteFileRecord) DeleteFileResult {
	if value == nil {
		return DeleteFileResult{}
	}

	return DeleteFileResult{
		FileID:                  value.File.FileID,
		Status:                  value.File.Status,
		DeletedAt:               cloneTimePtr(value.File.DeletedAt),
		PhysicalDeleteScheduled: value.PhysicalDeleteScheduled,
	}
}
