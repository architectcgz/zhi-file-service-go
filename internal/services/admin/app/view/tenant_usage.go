package view

import (
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
)

type Time time.Time

type TenantUsage struct {
	TenantID         string
	UsedStorageBytes int64
	UsedFileCount    int64
	LastUploadAt     *time.Time
	UpdatedAt        Time
}

func FromTenantUsage(value *ports.TenantUsageView) TenantUsage {
	if value == nil {
		return TenantUsage{}
	}

	return TenantUsage{
		TenantID:         value.TenantID,
		UsedStorageBytes: value.StorageBytes,
		UsedFileCount:    value.FileCount,
		LastUploadAt:     cloneTimePtr(value.LastUploadAt),
		UpdatedAt:        Time(value.UpdatedAt),
	}
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	cloned := value.UTC()
	return &cloned
}
