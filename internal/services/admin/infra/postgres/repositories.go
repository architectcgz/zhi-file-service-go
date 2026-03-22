package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	defaultMaxStorageBytes   int64 = 10 * 1024 * 1024 * 1024
	defaultMaxFileCount      int64 = 100000
	defaultMaxSingleFileSize int64 = 100 * 1024 * 1024
	outboxServiceName              = "admin-service"
)

var (
	defaultAllowedMimeTypes  = []string{"image/png", "image/jpeg", "application/pdf"}
	defaultAllowedExtensions = []string{"png", "jpg", "jpeg", "pdf"}
	defaultAccessLevel       = string(pkgstorage.AccessLevelPrivate)
	defaultAutoCreateEnabled = false
)

type TenantRepository struct {
	db *sql.DB
}

type TenantPolicyRepository struct {
	db *sql.DB
}

type TenantUsageRepository struct {
	db *sql.DB
}

type AdminFileRepository struct {
	db *sql.DB
}

type AuditLogRepository struct {
	db *sql.DB
}

type OutboxPublisher struct {
	db    *sql.DB
	idgen ids.Generator
}

type TxManager struct {
	db *sql.DB
}

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type txKey struct{}

type cursorValue struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
}

func NewTenantRepository(db *sql.DB) *TenantRepository {
	return &TenantRepository{db: assertDB(db)}
}

func NewTenantPolicyRepository(db *sql.DB) *TenantPolicyRepository {
	return &TenantPolicyRepository{db: assertDB(db)}
}

func NewTenantUsageRepository(db *sql.DB) *TenantUsageRepository {
	return &TenantUsageRepository{db: assertDB(db)}
}

func NewAdminFileRepository(db *sql.DB) *AdminFileRepository {
	return &AdminFileRepository{db: assertDB(db)}
}

func NewAuditLogRepository(db *sql.DB) *AuditLogRepository {
	return &AuditLogRepository{db: assertDB(db)}
}

func NewOutboxPublisher(db *sql.DB, idgen ids.Generator) *OutboxPublisher {
	if idgen == nil {
		idgen = ids.NewGenerator(nil, nil)
	}
	return &OutboxPublisher{db: assertDB(db), idgen: idgen}
}

func NewTxManager(db *sql.DB) TxManager {
	return TxManager{db: assertDB(db)}
}

func (r *TenantRepository) Create(ctx context.Context, tenant domain.Tenant) error {
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
INSERT INTO tenant.tenants (
  tenant_id, tenant_name, status, contact_email, description, created_at, updated_at, deleted_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)`,
		tenant.TenantID,
		tenant.TenantName,
		string(tenant.Status),
		nullString(tenant.ContactEmail),
		nullString(tenant.Description),
		tenant.CreatedAt.UTC(),
		tenant.UpdatedAt.UTC(),
		nullTime(statusDeletedAt(tenant.Status, tenant.UpdatedAt)),
	)
	if err != nil {
		return wrapDBError("create tenant", err)
	}
	return nil
}

func (r *TenantRepository) GetByID(ctx context.Context, tenantID string) (*domain.Tenant, error) {
	row := currentExecutor(ctx, r.db).QueryRowContext(ctx, `
SELECT tenant_id, tenant_name, status, contact_email, description, created_at, updated_at
FROM tenant.tenants
WHERE tenant_id = $1
`, tenantID)

	tenant, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDBError("get tenant", err)
	}
	return tenant, nil
}

func (r *TenantRepository) List(ctx context.Context, query ports.ListTenantsQuery) ([]domain.Tenant, string, error) {
	sqlQuery := `
SELECT tenant_id, tenant_name, status, contact_email, description, created_at, updated_at
FROM tenant.tenants
`
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 6)

	if len(query.TenantScopes) > 0 {
		args = append(args, query.TenantScopes)
		clauses = append(clauses, fmt.Sprintf("tenant_id = ANY($%d)", len(args)))
	}
	if query.Status != nil {
		args = append(args, string(*query.Status))
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if query.Cursor != "" {
		cursor, err := decodeCursor(query.Cursor)
		if err != nil {
			return nil, "", err
		}
		args = append(args, cursor.CreatedAt.UTC(), cursor.ID)
		clauses = append(clauses, fmt.Sprintf("(created_at, tenant_id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	if len(clauses) > 0 {
		sqlQuery += "WHERE " + strings.Join(clauses, " AND ") + "\n"
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit+1)
	sqlQuery += fmt.Sprintf("ORDER BY created_at DESC, tenant_id DESC LIMIT $%d", len(args))

	rows, err := currentExecutor(ctx, r.db).QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, "", wrapDBError("list tenants", err)
	}
	defer rows.Close()

	items := make([]domain.Tenant, 0, limit+1)
	for rows.Next() {
		item, err := scanTenant(rows)
		if err != nil {
			return nil, "", wrapDBError("scan tenant", err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", wrapDBError("iterate tenants", err)
	}

	return trimTenantPage(items, limit)
}

func (r *TenantRepository) Patch(ctx context.Context, tenantID string, patch domain.TenantPatch) (*domain.Tenant, error) {
	row := currentExecutor(ctx, r.db).QueryRowContext(ctx, `
UPDATE tenant.tenants
SET tenant_name = CASE WHEN $2 THEN $3 ELSE tenant_name END,
    status = CASE WHEN $4 THEN $5 ELSE status END,
    contact_email = CASE WHEN $6 THEN $7 ELSE contact_email END,
    description = CASE WHEN $8 THEN $9 ELSE description END,
    deleted_at = CASE
      WHEN $4 AND $5 = 'DELETED' THEN $10
      WHEN $4 AND $5 <> 'DELETED' THEN NULL
      ELSE deleted_at
    END,
    updated_at = $10
WHERE tenant_id = $1
RETURNING tenant_id, tenant_name, status, contact_email, description, created_at, updated_at
`,
		tenantID,
		patch.TenantName != nil, valueString(patch.TenantName),
		patch.Status != nil, valueTenantStatus(patch.Status),
		patch.ContactEmail != nil, valueString(patch.ContactEmail),
		patch.Description != nil, valueString(patch.Description),
		time.Now().UTC(),
	)

	tenant, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDBError("patch tenant", err)
	}
	return tenant, nil
}

func (r *TenantPolicyRepository) CreateDefault(ctx context.Context, tenantID string) error {
	now := time.Now().UTC()
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
INSERT INTO tenant.tenant_policies (
  tenant_id,
  max_storage_bytes,
  max_file_count,
  max_single_file_size,
  allowed_mime_types,
  allowed_extensions,
  default_access_level,
  auto_create_enabled,
  created_at,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)`,
		tenantID,
		defaultMaxStorageBytes,
		defaultMaxFileCount,
		defaultMaxSingleFileSize,
		defaultAllowedMimeTypes,
		defaultAllowedExtensions,
		defaultAccessLevel,
		defaultAutoCreateEnabled,
		now,
		now,
	)
	if err != nil {
		return wrapDBError("create default tenant policy", err)
	}
	return nil
}

func (r *TenantPolicyRepository) GetByTenantID(ctx context.Context, tenantID string) (*ports.TenantPolicyView, error) {
	row := currentExecutor(ctx, r.db).QueryRowContext(ctx, `
SELECT tenant_id, max_storage_bytes, max_file_count, max_single_file_size, allowed_mime_types,
       allowed_extensions, default_access_level, auto_create_enabled, created_at, updated_at
FROM tenant.tenant_policies
WHERE tenant_id = $1
`, tenantID)

	view, err := scanTenantPolicy(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDBError("get tenant policy", err)
	}
	return view, nil
}

func (r *TenantPolicyRepository) Patch(ctx context.Context, tenantID string, patch domain.TenantPolicyPatch) (*ports.TenantPolicyView, error) {
	row := currentExecutor(ctx, r.db).QueryRowContext(ctx, `
UPDATE tenant.tenant_policies
SET max_storage_bytes = CASE WHEN $2 THEN $3 ELSE max_storage_bytes END,
    max_file_count = CASE WHEN $4 THEN $5 ELSE max_file_count END,
    max_single_file_size = CASE WHEN $6 THEN $7 ELSE max_single_file_size END,
    allowed_mime_types = CASE WHEN $8 THEN $9 ELSE allowed_mime_types END,
    allowed_extensions = CASE WHEN $10 THEN $11 ELSE allowed_extensions END,
    default_access_level = CASE WHEN $12 THEN $13 ELSE default_access_level END,
    auto_create_enabled = CASE WHEN $14 THEN $15 ELSE auto_create_enabled END,
    updated_at = $16
WHERE tenant_id = $1
RETURNING tenant_id, max_storage_bytes, max_file_count, max_single_file_size, allowed_mime_types,
          allowed_extensions, default_access_level, auto_create_enabled, created_at, updated_at
`,
		tenantID,
		patch.MaxStorageBytes != nil, valueInt64(patch.MaxStorageBytes),
		patch.MaxFileCount != nil, valueInt64(patch.MaxFileCount),
		patch.MaxSingleFileSize != nil, valueInt64(patch.MaxSingleFileSize),
		patch.AllowedMimeTypes != nil, patch.AllowedMimeTypes,
		patch.AllowedExtensions != nil, patch.AllowedExtensions,
		patch.DefaultAccessLevel != nil, valueString(patch.DefaultAccessLevel),
		patch.AutoCreateEnabled != nil, valueBool(patch.AutoCreateEnabled),
		time.Now().UTC(),
	)

	view, err := scanTenantPolicy(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDBError("patch tenant policy", err)
	}
	return view, nil
}

func (r *TenantUsageRepository) Initialize(ctx context.Context, tenantID string) error {
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
INSERT INTO tenant.tenant_usage (
  tenant_id, used_storage_bytes, used_file_count, last_upload_at, version, updated_at
) VALUES (
  $1, 0, 0, NULL, 0, $2
)`,
		tenantID,
		time.Now().UTC(),
	)
	if err != nil {
		return wrapDBError("initialize tenant usage", err)
	}
	return nil
}

func (r *TenantUsageRepository) GetByTenantID(ctx context.Context, tenantID string) (*ports.TenantUsageView, error) {
	row := currentExecutor(ctx, r.db).QueryRowContext(ctx, `
SELECT tenant_id, used_storage_bytes, used_file_count, last_upload_at, updated_at
FROM tenant.tenant_usage
WHERE tenant_id = $1
`, tenantID)

	view, err := scanTenantUsage(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDBError("get tenant usage", err)
	}
	return view, nil
}

func (r *AdminFileRepository) GetByID(ctx context.Context, fileID string) (*ports.AdminFileView, error) {
	row := currentExecutor(ctx, r.db).QueryRowContext(ctx, `
SELECT file_id, tenant_id, owner_id, blob_object_id, original_filename, content_type, file_size,
       access_level, status, deleted_at, created_at, updated_at
FROM file.file_assets
WHERE file_id = $1
`, fileID)

	file, err := scanAdminFile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDBError("get admin file", err)
	}
	return file, nil
}

func (r *AdminFileRepository) List(ctx context.Context, query ports.ListFilesQuery) ([]ports.AdminFileView, string, error) {
	sqlQuery := `
SELECT file_id, tenant_id, owner_id, blob_object_id, original_filename, content_type, file_size,
       access_level, status, deleted_at, created_at, updated_at
FROM file.file_assets
`
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 6)

	if query.TenantID != "" {
		args = append(args, query.TenantID)
		clauses = append(clauses, fmt.Sprintf("tenant_id = $%d", len(args)))
	} else if len(query.TenantScopes) > 0 {
		args = append(args, query.TenantScopes)
		clauses = append(clauses, fmt.Sprintf("tenant_id = ANY($%d)", len(args)))
	}
	if query.Status != "" {
		args = append(args, query.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if query.Cursor != "" {
		cursor, err := decodeCursor(query.Cursor)
		if err != nil {
			return nil, "", err
		}
		args = append(args, cursor.CreatedAt.UTC(), cursor.ID)
		clauses = append(clauses, fmt.Sprintf("(created_at, file_id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	if len(clauses) > 0 {
		sqlQuery += "WHERE " + strings.Join(clauses, " AND ") + "\n"
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit+1)
	sqlQuery += fmt.Sprintf("ORDER BY created_at DESC, file_id DESC LIMIT $%d", len(args))

	rows, err := currentExecutor(ctx, r.db).QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, "", wrapDBError("list admin files", err)
	}
	defer rows.Close()

	items := make([]ports.AdminFileView, 0, limit+1)
	for rows.Next() {
		item, err := scanAdminFile(rows)
		if err != nil {
			return nil, "", wrapDBError("scan admin file", err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", wrapDBError("iterate admin files", err)
	}

	return trimAdminFilePage(items, limit)
}

func (r *AdminFileRepository) MarkDeleted(ctx context.Context, fileID string, deletedAt time.Time) (*ports.DeleteFileRecord, error) {
	row := currentExecutor(ctx, r.db).QueryRowContext(ctx, `
SELECT file_id, tenant_id, owner_id, blob_object_id, original_filename, content_type, file_size,
       access_level, status, deleted_at, created_at, updated_at
FROM file.file_assets
WHERE file_id = $1
FOR UPDATE
`, fileID)

	current, err := scanAdminFile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDBError("lock admin file", err)
	}
	if current.Status == "DELETED" {
		return &ports.DeleteFileRecord{
			File:                    *current,
			PhysicalDeleteScheduled: true,
			AlreadyDeleted:          true,
		}, nil
	}

	deletedAt = deletedAt.UTC()
	row = currentExecutor(ctx, r.db).QueryRowContext(ctx, `
UPDATE file.file_assets
SET status = 'DELETED',
    deleted_at = $2,
    updated_at = $2
WHERE file_id = $1
RETURNING file_id, tenant_id, owner_id, blob_object_id, original_filename, content_type, file_size,
          access_level, status, deleted_at, created_at, updated_at
`, fileID, deletedAt)

	deleted, err := scanAdminFile(row)
	if err != nil {
		return nil, wrapDBError("mark admin file deleted", err)
	}
	if _, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
UPDATE file.blob_objects
SET reference_count = GREATEST(reference_count - 1, 0),
    updated_at = $2
WHERE blob_object_id = $1
`, deleted.BlobID, deletedAt); err != nil {
		return nil, wrapDBError("decrement blob reference count", err)
	}
	if _, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
UPDATE tenant.tenant_usage
SET used_storage_bytes = GREATEST(used_storage_bytes - $2, 0),
    used_file_count = GREATEST(used_file_count - 1, 0),
    version = version + 1,
    updated_at = $3
WHERE tenant_id = $1
`, deleted.TenantID, deleted.SizeBytes, deletedAt); err != nil {
		return nil, wrapDBError("apply tenant usage delete delta", err)
	}

	return &ports.DeleteFileRecord{
		File:                    *deleted,
		PhysicalDeleteScheduled: true,
	}, nil
}

func (r *AuditLogRepository) Append(ctx context.Context, record ports.AuditLogRecord) error {
	details, err := marshalJSON(record.Details)
	if err != nil {
		return xerrors.Wrap(xerrors.CodeInternalError, "marshal admin audit details", err, nil)
	}

	_, err = currentExecutor(ctx, r.db).ExecContext(ctx, `
INSERT INTO audit.admin_audit_logs (
  audit_log_id, admin_subject, action, target_type, target_id, tenant_id, request_id,
  ip_address, user_agent, details, created_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, NULL, NULL, $8, $9
)`,
		record.AuditID,
		record.AdminSubject,
		record.Action,
		record.TargetType,
		nullString(record.TargetID),
		nullString(record.TenantID),
		nullString(record.RequestID),
		details,
		record.CreatedAt.UTC(),
	)
	if err != nil {
		return wrapDBError("append admin audit log", err)
	}
	return nil
}

func (r *AuditLogRepository) List(ctx context.Context, query ports.ListAuditLogsQuery) ([]ports.AuditLogRecord, string, error) {
	sqlQuery := `
SELECT audit_log_id, admin_subject, tenant_id, action, target_type, target_id, request_id, details, created_at
FROM audit.admin_audit_logs
`
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 7)

	if query.TenantID != "" {
		args = append(args, query.TenantID)
		clauses = append(clauses, fmt.Sprintf("tenant_id = $%d", len(args)))
	} else if len(query.TenantScopes) > 0 {
		args = append(args, query.TenantScopes)
		clauses = append(clauses, fmt.Sprintf("tenant_id = ANY($%d)", len(args)))
	}
	if query.ActorID != "" {
		args = append(args, query.ActorID)
		clauses = append(clauses, fmt.Sprintf("admin_subject = $%d", len(args)))
	}
	if query.Action != "" {
		args = append(args, query.Action)
		clauses = append(clauses, fmt.Sprintf("action = $%d", len(args)))
	}
	if query.Cursor != "" {
		cursor, err := decodeCursor(query.Cursor)
		if err != nil {
			return nil, "", err
		}
		args = append(args, cursor.CreatedAt.UTC(), cursor.ID)
		clauses = append(clauses, fmt.Sprintf("(created_at, audit_log_id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	if len(clauses) > 0 {
		sqlQuery += "WHERE " + strings.Join(clauses, " AND ") + "\n"
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit+1)
	sqlQuery += fmt.Sprintf("ORDER BY created_at DESC, audit_log_id DESC LIMIT $%d", len(args))

	rows, err := currentExecutor(ctx, r.db).QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, "", wrapDBError("list admin audit logs", err)
	}
	defer rows.Close()

	items := make([]ports.AuditLogRecord, 0, limit+1)
	for rows.Next() {
		item, err := scanAuditLog(rows)
		if err != nil {
			return nil, "", wrapDBError("scan admin audit log", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", wrapDBError("iterate admin audit logs", err)
	}

	return trimAuditPage(items, limit)
}

func (p *OutboxPublisher) Publish(ctx context.Context, event ports.OutboxEvent) error {
	eventID, err := p.idgen.New()
	if err != nil {
		return xerrors.Wrap(xerrors.CodeInternalError, "generate outbox event id", err, nil)
	}

	payload, err := marshalJSON(event.Payload)
	if err != nil {
		return xerrors.Wrap(xerrors.CodeInternalError, "marshal outbox payload", err, nil)
	}

	now := event.OccurredAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	_, err = currentExecutor(ctx, p.db).ExecContext(ctx, `
INSERT INTO infra.outbox_events (
  event_id, service_name, aggregate_type, aggregate_id, event_type, payload, status,
  retry_count, next_attempt_at, published_at, created_at, updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, 'PENDING', 0, $7, NULL, $7, $7
)`,
		eventID,
		outboxServiceName,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		payload,
		now,
	)
	if err != nil {
		return wrapDBError("publish outbox event", err)
	}
	return nil
}

func (m TxManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if currentTx(ctx) != nil {
		return fn(ctx)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return wrapDBError("begin transaction", err)
	}
	txCtx := withTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return wrapDBError("commit transaction", err)
	}
	return nil
}

func currentExecutor(ctx context.Context, db *sql.DB) executor {
	if tx := currentTx(ctx); tx != nil {
		return tx
	}
	return db
}

func withTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func currentTx(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(txKey{}).(*sql.Tx)
	return tx
}

func scanTenant(scanner interface{ Scan(...any) error }) (*domain.Tenant, error) {
	var (
		tenant      domain.Tenant
		status      string
		email       sql.NullString
		description sql.NullString
	)
	if err := scanner.Scan(
		&tenant.TenantID,
		&tenant.TenantName,
		&status,
		&email,
		&description,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	); err != nil {
		return nil, err
	}
	tenant.Status = domain.TenantStatus(status)
	tenant.ContactEmail = email.String
	tenant.Description = description.String
	tenant.CreatedAt = tenant.CreatedAt.UTC()
	tenant.UpdatedAt = tenant.UpdatedAt.UTC()
	return &tenant, nil
}

func scanTenantPolicy(scanner interface{ Scan(...any) error }) (*ports.TenantPolicyView, error) {
	var (
		view               ports.TenantPolicyView
		maxStorageBytes    int64
		maxFileCount       int64
		maxSingleFileSize  int64
		allowedMimeTypes   []string
		allowedExtensions  []string
		defaultAccessLevel string
		autoCreateEnabled  bool
	)
	if err := scanner.Scan(
		&view.TenantID,
		&maxStorageBytes,
		&maxFileCount,
		&maxSingleFileSize,
		&allowedMimeTypes,
		&allowedExtensions,
		&defaultAccessLevel,
		&autoCreateEnabled,
		&view.CreatedAt,
		&view.UpdatedAt,
	); err != nil {
		return nil, err
	}
	view.Policy = domain.TenantPolicy{
		MaxStorageBytes:    int64Ptr(maxStorageBytes),
		MaxFileCount:       int64Ptr(maxFileCount),
		MaxSingleFileSize:  int64Ptr(maxSingleFileSize),
		AllowedMimeTypes:   cloneStrings(allowedMimeTypes),
		AllowedExtensions:  cloneStrings(allowedExtensions),
		DefaultAccessLevel: stringPtr(defaultAccessLevel),
		AutoCreateEnabled:  boolPtr(autoCreateEnabled),
	}.Normalize()
	view.CreatedAt = view.CreatedAt.UTC()
	view.UpdatedAt = view.UpdatedAt.UTC()
	return &view, nil
}

func scanTenantUsage(scanner interface{ Scan(...any) error }) (*ports.TenantUsageView, error) {
	var (
		view         ports.TenantUsageView
		lastUploadAt sql.NullTime
	)
	if err := scanner.Scan(
		&view.TenantID,
		&view.StorageBytes,
		&view.FileCount,
		&lastUploadAt,
		&view.UpdatedAt,
	); err != nil {
		return nil, err
	}
	view.LastUploadAt = timePtr(lastUploadAt)
	view.UpdatedAt = view.UpdatedAt.UTC()
	return &view, nil
}

func scanAdminFile(scanner interface{ Scan(...any) error }) (*ports.AdminFileView, error) {
	var (
		file        ports.AdminFileView
		contentType sql.NullString
		deletedAt   sql.NullTime
		accessLevel string
	)
	if err := scanner.Scan(
		&file.FileID,
		&file.TenantID,
		&file.OwnerID,
		&file.BlobID,
		&file.FileName,
		&contentType,
		&file.SizeBytes,
		&accessLevel,
		&file.Status,
		&deletedAt,
		&file.CreatedAt,
		&file.UpdatedAt,
	); err != nil {
		return nil, err
	}
	file.ContentType = contentType.String
	file.AccessLevel = pkgstorage.AccessLevel(accessLevel)
	file.DeletedAt = timePtr(deletedAt)
	file.CreatedAt = file.CreatedAt.UTC()
	file.UpdatedAt = file.UpdatedAt.UTC()
	return &file, nil
}

func scanAuditLog(scanner interface{ Scan(...any) error }) (ports.AuditLogRecord, error) {
	var (
		record    ports.AuditLogRecord
		tenantID  sql.NullString
		targetID  sql.NullString
		requestID sql.NullString
		details   []byte
	)
	if err := scanner.Scan(
		&record.AuditID,
		&record.AdminSubject,
		&tenantID,
		&record.Action,
		&record.TargetType,
		&targetID,
		&requestID,
		&details,
		&record.CreatedAt,
	); err != nil {
		return ports.AuditLogRecord{}, err
	}
	record.TenantID = tenantID.String
	record.TargetID = targetID.String
	record.RequestID = requestID.String
	record.CreatedAt = record.CreatedAt.UTC()
	if len(details) > 0 {
		if err := json.Unmarshal(details, &record.Details); err != nil {
			return ports.AuditLogRecord{}, xerrors.Wrap(xerrors.CodeInternalError, "unmarshal admin audit details", err, nil)
		}
	}
	return record, nil
}

func trimTenantPage(items []domain.Tenant, limit int) ([]domain.Tenant, string, error) {
	if len(items) <= limit {
		return items, "", nil
	}
	next := items[limit]
	cursor, err := encodeCursor(next.CreatedAt, next.TenantID)
	if err != nil {
		return nil, "", err
	}
	return items[:limit], cursor, nil
}

func trimAdminFilePage(items []ports.AdminFileView, limit int) ([]ports.AdminFileView, string, error) {
	if len(items) <= limit {
		return items, "", nil
	}
	next := items[limit]
	cursor, err := encodeCursor(next.CreatedAt, next.FileID)
	if err != nil {
		return nil, "", err
	}
	return items[:limit], cursor, nil
}

func trimAuditPage(items []ports.AuditLogRecord, limit int) ([]ports.AuditLogRecord, string, error) {
	if len(items) <= limit {
		return items, "", nil
	}
	next := items[limit]
	cursor, err := encodeCursor(next.CreatedAt, next.AuditID)
	if err != nil {
		return nil, "", err
	}
	return items[:limit], cursor, nil
}

func encodeCursor(createdAt time.Time, id string) (string, error) {
	payload, err := json.Marshal(cursorValue{
		CreatedAt: createdAt.UTC(),
		ID:        strings.TrimSpace(id),
	})
	if err != nil {
		return "", xerrors.Wrap(xerrors.CodeInternalError, "marshal cursor", err, nil)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(value string) (cursorValue, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return cursorValue{}, xerrors.New(xerrors.CodeInvalidArgument, "query parameter is invalid", xerrors.Details{
			"field":  "cursor",
			"reason": "invalid_cursor",
		})
	}
	var cursor cursorValue
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.CreatedAt.IsZero() || strings.TrimSpace(cursor.ID) == "" {
		return cursorValue{}, xerrors.New(xerrors.CodeInvalidArgument, "query parameter is invalid", xerrors.Details{
			"field":  "cursor",
			"reason": "invalid_cursor",
		})
	}
	cursor.CreatedAt = cursor.CreatedAt.UTC()
	cursor.ID = strings.TrimSpace(cursor.ID)
	return cursor, nil
}

func marshalJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func wrapDBError(message string, err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return xerrors.Wrap(xerrors.CodeConflict, message, err, nil)
		}
	}
	return xerrors.Wrap(xerrors.CodeServiceUnavailable, message, err, nil)
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func valueString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func valueTenantStatus(value *domain.TenantStatus) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func valueInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func valueBool(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func statusDeletedAt(status domain.TenantStatus, updatedAt time.Time) *time.Time {
	if status != domain.TenantStatusDeleted {
		return nil
	}
	value := updatedAt.UTC()
	return &value
}

func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed := value.Time.UTC()
	return &parsed
}

func int64Ptr(value int64) *int64 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, value)
	}
	return cloned
}

func assertDB(db *sql.DB) *sql.DB {
	if db == nil {
		panic("admin postgres repository requires a non-nil *sql.DB")
	}
	return db
}

var (
	_ ports.TenantRepository       = (*TenantRepository)(nil)
	_ ports.TenantPolicyRepository = (*TenantPolicyRepository)(nil)
	_ ports.TenantUsageRepository  = (*TenantUsageRepository)(nil)
	_ ports.AdminFileRepository    = (*AdminFileRepository)(nil)
	_ ports.AuditLogRepository     = (*AuditLogRepository)(nil)
	_ ports.OutboxPublisher        = (*OutboxPublisher)(nil)
)
