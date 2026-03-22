package ports

import "context"

type TenantUploadPolicy struct {
	AllowInlineUpload bool
	AllowMultipart    bool
	MaxInlineSize     int64
	MaxFileSize       int64
	AllowedMimeTypes  []string
}

type TenantPolicyReader interface {
	ReadUploadPolicy(ctx context.Context, tenantID string) (TenantUploadPolicy, error)
}
