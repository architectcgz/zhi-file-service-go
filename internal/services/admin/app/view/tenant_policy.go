package view

import "github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"

type TenantPolicy struct {
	TenantID           string
	MaxStorageBytes    *int64
	MaxFileCount       *int64
	MaxSingleFileSize  *int64
	AllowedMimeTypes   []string
	AllowedExtensions  []string
	DefaultAccessLevel *string
	AutoCreateEnabled  *bool
	CreatedAt          Time
	UpdatedAt          Time
}

func FromTenantPolicy(value *ports.TenantPolicyView) TenantPolicy {
	if value == nil {
		return TenantPolicy{}
	}

	policy := value.Policy.Normalize()
	return TenantPolicy{
		TenantID:           value.TenantID,
		MaxStorageBytes:    policy.MaxStorageBytes,
		MaxFileCount:       policy.MaxFileCount,
		MaxSingleFileSize:  policy.MaxSingleFileSize,
		AllowedMimeTypes:   policy.AllowedMimeTypes,
		AllowedExtensions:  policy.AllowedExtensions,
		DefaultAccessLevel: policy.DefaultAccessLevel,
		AutoCreateEnabled:  policy.AutoCreateEnabled,
		CreatedAt:          Time(value.CreatedAt),
		UpdatedAt:          Time(value.UpdatedAt),
	}
}
