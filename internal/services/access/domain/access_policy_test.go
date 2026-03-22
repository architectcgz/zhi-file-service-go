package domain_test

import (
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestFileViewEnsureReadable(t *testing.T) {
	timestamp := time.Date(2026, 3, 22, 8, 0, 0, 0, time.UTC)
	file := domain.FileView{
		FileID:          "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
		TenantID:        "tenant-a",
		FileName:        "avatar.png",
		AccessLevel:     storage.AccessLevelPublic,
		Status:          domain.FileStatusActive,
		StorageProvider: storage.ProviderS3,
		BucketName:      "public",
		ObjectKey:       "tenant-a/avatar.png",
		CreatedAt:       timestamp,
		UpdatedAt:       timestamp,
	}

	tests := []struct {
		name string
		auth domain.AuthContext
		want xerrors.Code
		mut  func(*domain.FileView)
	}{
		{
			name: "same tenant can read active file",
			auth: domain.AuthContext{
				SubjectID: "user-1",
				TenantID:  "tenant-a",
				Scopes:    []string{domain.ScopeFileRead},
			},
		},
		{
			name: "cross tenant denied",
			auth: domain.AuthContext{
				SubjectID: "user-1",
				TenantID:  "tenant-b",
				Scopes:    []string{domain.ScopeFileRead},
			},
			want: domain.CodeTenantScopeDenied,
		},
		{
			name: "deleted file denied",
			auth: domain.AuthContext{
				SubjectID: "user-1",
				TenantID:  "tenant-a",
				Scopes:    []string{domain.ScopeFileRead},
			},
			want: domain.CodeFileAccessDenied,
			mut: func(file *domain.FileView) {
				file.Status = domain.FileStatusDeleted
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := file
			if tt.mut != nil {
				tt.mut(&current)
			}

			err := current.EnsureReadable(tt.auth)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("EnsureReadable returned error: %v", err)
				}
				return
			}
			if xerrors.CodeOf(err) != tt.want {
				t.Fatalf("expected code %s, got %s (err=%v)", tt.want, xerrors.CodeOf(err), err)
			}
		})
	}
}
