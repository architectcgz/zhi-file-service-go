package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type FileReadRepository struct {
	db *sql.DB
}

type TenantPolicyReader struct {
	db *sql.DB
}

func NewFileReadRepository(db *sql.DB) *FileReadRepository {
	return &FileReadRepository{db: db}
}

func NewTenantPolicyReader(db *sql.DB) *TenantPolicyReader {
	return &TenantPolicyReader{db: db}
}

func (r *FileReadRepository) GetByID(ctx context.Context, fileID string) (domain.FileView, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT file_id, tenant_id, owner_id, original_filename, content_type, file_size,
       access_level, status, storage_provider, bucket_name, object_key, created_at, updated_at
FROM file.file_assets
WHERE file_id = $1
`, fileID)

	var (
		file        domain.FileView
		contentType sql.NullString
		accessLevel string
		status      string
		provider    string
		createdAt   time.Time
		updatedAt   time.Time
	)
	if err := row.Scan(
		&file.FileID,
		&file.TenantID,
		&file.OwnerID,
		&file.FileName,
		&contentType,
		&file.SizeBytes,
		&accessLevel,
		&status,
		&provider,
		&file.BucketName,
		&file.ObjectKey,
		&createdAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.FileView{}, domain.ErrFileNotFound(fileID)
		}
		return domain.FileView{}, wrapDBError("get file by id", err)
	}

	file.ContentType = contentType.String
	file.AccessLevel = storage.AccessLevel(accessLevel)
	file.Status = domain.FileStatus(status)
	file.StorageProvider = storage.Provider(provider)
	file.CreatedAt = createdAt.UTC()
	file.UpdatedAt = updatedAt.UTC()

	return file, nil
}

func (r *TenantPolicyReader) GetByTenantID(ctx context.Context, tenantID string) (domain.TenantPolicy, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT tenant_id
FROM tenant.tenant_policies
WHERE tenant_id = $1
`, tenantID)

	var policy domain.TenantPolicy
	if err := row.Scan(&policy.TenantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TenantPolicy{}, xerrors.New(xerrors.CodeForbidden, "tenant policy is not configured", xerrors.Details{
				"tenantId": tenantID,
			})
		}
		return domain.TenantPolicy{}, wrapDBError("get tenant policy by tenant id", err)
	}

	return policy, nil
}

func wrapDBError(message string, err error) error {
	if err == nil {
		return nil
	}
	return xerrors.Wrap(xerrors.CodeServiceUnavailable, message, err, nil)
}

var (
	_ ports.FileReadRepository = (*FileReadRepository)(nil)
	_ ports.TenantPolicyReader = (*TenantPolicyReader)(nil)
)
