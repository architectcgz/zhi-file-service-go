package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

const outboxClaimLease = 5 * time.Minute

type ObjectStorage interface {
	DeleteObject(context.Context, pkgstorage.ObjectRef) error
	AbortMultipartUpload(context.Context, pkgstorage.ObjectRef, string) error
}

type UploadSessionRepository struct {
	db *sql.DB
}

type FileCleanupRepository struct {
	db      *sql.DB
	storage ObjectStorage
}

type BlobRepository struct {
	db      *sql.DB
	storage ObjectStorage
}

type MultipartRepository struct {
	db      *sql.DB
	storage ObjectStorage
}

type TenantUsageRepository struct {
	db *sql.DB
}

type OutboxReader struct {
	db *sql.DB
}

func NewUploadSessionRepository(db *sql.DB) *UploadSessionRepository {
	return &UploadSessionRepository{db: db}
}

func NewFileCleanupRepository(db *sql.DB, storage ObjectStorage) *FileCleanupRepository {
	return &FileCleanupRepository{db: db, storage: storage}
}

func NewBlobRepository(db *sql.DB, storage ObjectStorage) *BlobRepository {
	return &BlobRepository{db: db, storage: storage}
}

func NewMultipartRepository(db *sql.DB, storage ObjectStorage) *MultipartRepository {
	return &MultipartRepository{db: db, storage: storage}
}

func NewTenantUsageRepository(db *sql.DB) *TenantUsageRepository {
	return &TenantUsageRepository{db: db}
}

func NewOutboxReader(db *sql.DB) *OutboxReader {
	return &OutboxReader{db: db}
}

func (r *UploadSessionRepository) ExpirePendingSessions(ctx context.Context, expiredBefore time.Time, limit int) (int, error) {
	result, err := r.db.ExecContext(ctx, `
WITH candidates AS (
	SELECT upload_session_id
	FROM upload.upload_sessions
	WHERE status IN ('INITIATED', 'UPLOADING', 'COMPLETING')
	  AND expires_at <= $1
	ORDER BY expires_at ASC
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)
UPDATE upload.upload_sessions AS s
SET status = 'EXPIRED',
    updated_at = NOW(),
    completion_token = NULL,
    completion_started_at = NULL
FROM candidates
WHERE s.upload_session_id = candidates.upload_session_id
`, expiredBefore.UTC(), normalizeLimit(limit))
	if err != nil {
		return 0, err
	}
	return rowsAffected(result)
}

func (r *UploadSessionRepository) RepairStuckCompleting(ctx context.Context, staleBefore time.Time, limit int) (int, error) {
	result, err := r.db.ExecContext(ctx, `
WITH candidates AS (
	SELECT upload_session_id, file_id
	FROM upload.upload_sessions
	WHERE status = 'COMPLETING'
	  AND updated_at <= $1
	ORDER BY updated_at ASC
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)
UPDATE upload.upload_sessions AS s
SET status = CASE
		WHEN COALESCE(candidates.file_id, '') <> '' THEN 'COMPLETED'
		ELSE 'FAILED'
	END,
    completed_at = CASE
		WHEN COALESCE(candidates.file_id, '') <> '' THEN COALESCE(s.completed_at, NOW())
		ELSE s.completed_at
	END,
    failed_at = CASE
		WHEN COALESCE(candidates.file_id, '') = '' THEN NOW()
		ELSE s.failed_at
	END,
    failure_code = CASE
		WHEN COALESCE(candidates.file_id, '') = '' THEN 'JOB_REPAIR_STUCK_COMPLETING'
		ELSE NULL
	END,
    failure_message = CASE
		WHEN COALESCE(candidates.file_id, '') = '' THEN 'repair stuck completing session failed'
		ELSE NULL
	END,
    updated_at = NOW()
FROM candidates
WHERE s.upload_session_id = candidates.upload_session_id
`, staleBefore.UTC(), normalizeLimit(limit))
	if err != nil {
		return 0, err
	}
	return rowsAffected(result)
}

func (r *FileCleanupRepository) FinalizeDeletedFiles(ctx context.Context, eligibleBefore time.Time, limit int) (int, error) {
	if r.storage == nil {
		return 0, fmt.Errorf("object storage is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, `
WITH candidates AS (
	SELECT f.file_id,
	       b.blob_object_id,
	       b.storage_provider,
	       b.bucket_name,
	       b.object_key,
	       b.reference_count,
	       b.deleted_at
	FROM file.file_assets AS f
	JOIN file.blob_objects AS b ON b.blob_object_id = f.blob_object_id
	WHERE f.status = 'DELETED'
	  AND f.deleted_at IS NOT NULL
	  AND f.deleted_at <= $1
	  AND b.reference_count = 0
	  AND b.deleted_at IS NULL
	ORDER BY f.deleted_at ASC
	LIMIT $2
	FOR UPDATE OF f, b SKIP LOCKED
)
SELECT file_id, blob_object_id, storage_provider, bucket_name, object_key, reference_count, deleted_at
FROM candidates
`, eligibleBefore.UTC(), normalizeLimit(limit))
	if err != nil {
		return 0, err
	}

	records, err := scanCleanupRecords(rows)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	processed := 0
	for _, record := range records {
		if err := r.storage.DeleteObject(ctx, record.ObjectRef()); err != nil && !strings.Contains(err.Error(), pkgstorage.ErrObjectNotFound.Error()) {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE file.blob_objects
SET deleted_at = COALESCE(deleted_at, $1),
    updated_at = $1
WHERE blob_object_id = $2
  AND reference_count = 0
`, now, record.BlobObjectID); err != nil {
			return 0, err
		}
		processed++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return processed, nil
}

func (r *BlobRepository) CleanupOrphanBlobs(ctx context.Context, staleBefore time.Time, limit int) (int, error) {
	if r.storage == nil {
		return 0, fmt.Errorf("object storage is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, `
WITH candidates AS (
	SELECT b.blob_object_id,
	       b.storage_provider,
	       b.bucket_name,
	       b.object_key,
	       b.reference_count,
	       b.deleted_at
	FROM file.blob_objects AS b
	WHERE b.reference_count = 0
	  AND b.deleted_at IS NULL
	  AND b.updated_at <= $1
	  AND NOT EXISTS (
		SELECT 1
		FROM file.file_assets AS f
		WHERE f.blob_object_id = b.blob_object_id
		  AND f.status = 'ACTIVE'
	  )
	ORDER BY b.updated_at ASC
	LIMIT $2
	FOR UPDATE OF b SKIP LOCKED
)
SELECT '' AS file_id, blob_object_id, storage_provider, bucket_name, object_key, reference_count, deleted_at
FROM candidates
`, staleBefore.UTC(), normalizeLimit(limit))
	if err != nil {
		return 0, err
	}

	records, err := scanCleanupRecords(rows)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	processed := 0
	for _, record := range records {
		if err := r.storage.DeleteObject(ctx, record.ObjectRef()); err != nil && !strings.Contains(err.Error(), pkgstorage.ErrObjectNotFound.Error()) {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE file.blob_objects
SET deleted_at = COALESCE(deleted_at, $1),
    updated_at = $1
WHERE blob_object_id = $2
  AND reference_count = 0
`, now, record.BlobObjectID); err != nil {
			return 0, err
		}
		processed++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return processed, nil
}

func (r *MultipartRepository) CleanupMultipartUploads(ctx context.Context, staleBefore time.Time, limit int) (int, error) {
	if r.storage == nil {
		return 0, fmt.Errorf("object storage is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, `
SELECT upload_session_id, storage_provider, bucket_name, object_key, provider_upload_id
FROM upload.upload_sessions
WHERE provider_upload_id IS NOT NULL
  AND provider_upload_id <> ''
  AND updated_at <= $1
  AND status IN ('ABORTED', 'EXPIRED', 'FAILED')
ORDER BY updated_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED
`, staleBefore.UTC(), normalizeLimit(limit))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type multipartRecord struct {
		UploadSessionID string
		StorageProvider string
		BucketName      string
		ObjectKey       string
		ProviderUpload  string
	}

	var records []multipartRecord
	for rows.Next() {
		var record multipartRecord
		if err := rows.Scan(&record.UploadSessionID, &record.StorageProvider, &record.BucketName, &record.ObjectKey, &record.ProviderUpload); err != nil {
			return 0, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	processed := 0
	for _, record := range records {
		if err := r.storage.AbortMultipartUpload(ctx, pkgstorage.ObjectRef{
			Provider:   pkgstorage.Provider(record.StorageProvider),
			BucketName: record.BucketName,
			ObjectKey:  record.ObjectKey,
		}, record.ProviderUpload); err != nil && !strings.Contains(err.Error(), pkgstorage.ErrObjectNotFound.Error()) {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE upload.upload_sessions
SET provider_upload_id = NULL,
    updated_at = NOW()
WHERE upload_session_id = $1
`, record.UploadSessionID); err != nil {
			return 0, err
		}
		processed++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return processed, nil
}

func (r *TenantUsageRepository) ReconcileTenantUsage(ctx context.Context, limit int) (int, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT tenant_id
FROM tenant.tenant_usage
ORDER BY updated_at ASC
LIMIT $1
`, normalizeLimit(limit))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var tenantIDs []string
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return 0, err
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	for _, tenantID := range tenantIDs {
		if _, err := r.db.ExecContext(ctx, `
UPDATE tenant.tenant_usage
SET used_storage_bytes = COALESCE((
		SELECT SUM(file_size)
		FROM file.file_assets
		WHERE tenant_id = $1
		  AND status = 'ACTIVE'
	), 0),
    used_file_count = COALESCE((
		SELECT COUNT(*)
		FROM file.file_assets
		WHERE tenant_id = $1
		  AND status = 'ACTIVE'
	), 0),
    last_upload_at = (
		SELECT MAX(created_at)
		FROM file.file_assets
		WHERE tenant_id = $1
		  AND status = 'ACTIVE'
	),
    version = version + 1,
    updated_at = $2
WHERE tenant_id = $1
`, tenantID, now); err != nil {
			return 0, err
		}
	}

	return len(tenantIDs), nil
}

func (r *OutboxReader) ClaimPending(ctx context.Context, query ports.ClaimOutboxEventsQuery) ([]ports.OutboxEvent, error) {
	limit := normalizeLimit(query.Limit)
	claimUntil := query.DueBefore.UTC().Add(outboxClaimLease)

	var builder strings.Builder
	builder.WriteString(`
WITH candidates AS (
	SELECT event_id
	FROM infra.outbox_events
	WHERE status IN ('PENDING', 'FAILED')
	  AND COALESCE(next_attempt_at, created_at) <= $1
`)

	args := []any{query.DueBefore.UTC()}
	if len(query.EventTypes) > 0 {
		builder.WriteString("  AND event_type IN (")
		for idx, eventType := range query.EventTypes {
			if idx > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(fmt.Sprintf("$%d", len(args)+1))
			args = append(args, strings.TrimSpace(eventType))
		}
		builder.WriteString(")\n")
	}

	claimUntilArg := len(args) + 1
	limitArg := len(args) + 2
	builder.WriteString(fmt.Sprintf(`ORDER BY created_at ASC
	LIMIT $%d
	FOR UPDATE SKIP LOCKED
),
claimed AS (
	UPDATE infra.outbox_events AS e
	SET next_attempt_at = $%d,
	    updated_at = NOW()
	FROM candidates
	WHERE e.event_id = candidates.event_id
	RETURNING e.event_id, e.service_name, e.aggregate_type, e.aggregate_id, e.event_type, e.payload, e.retry_count, e.next_attempt_at
)
SELECT event_id, service_name, aggregate_type, aggregate_id, event_type, payload, retry_count, next_attempt_at
FROM claimed
ORDER BY event_id
`, limitArg, claimUntilArg))

	args = append(args, claimUntil, limit)
	rows, err := r.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ports.OutboxEvent
	for rows.Next() {
		var (
			event         ports.OutboxEvent
			payloadRaw    []byte
			nextAttemptAt sql.NullTime
		)
		if err := rows.Scan(
			&event.EventID,
			&event.ServiceName,
			&event.AggregateType,
			&event.AggregateID,
			&event.EventType,
			&payloadRaw,
			&event.RetryCount,
			&nextAttemptAt,
		); err != nil {
			return nil, err
		}
		if len(payloadRaw) > 0 {
			if err := json.Unmarshal(payloadRaw, &event.Payload); err != nil {
				return nil, err
			}
		}
		if nextAttemptAt.Valid {
			value := nextAttemptAt.Time.UTC()
			event.NextAttemptAt = &value
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *OutboxReader) MarkPublished(ctx context.Context, eventID string, publishedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE infra.outbox_events
SET status = 'PUBLISHED',
    published_at = $2,
    updated_at = $2
WHERE event_id = $1
`, strings.TrimSpace(eventID), publishedAt.UTC())
	return err
}

func (r *OutboxReader) MarkFailed(ctx context.Context, eventID string, nextAttemptAt time.Time, lastError string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE infra.outbox_events
SET status = 'FAILED',
    retry_count = retry_count + 1,
    next_attempt_at = $2,
    updated_at = $2
WHERE event_id = $1
`, strings.TrimSpace(eventID), nextAttemptAt.UTC())
	return err
}

type cleanupRecord struct {
	FileID          string
	BlobObjectID    string
	StorageProvider string
	BucketName      string
	ObjectKey       string
	ReferenceCount  int
}

func (r cleanupRecord) ObjectRef() pkgstorage.ObjectRef {
	return pkgstorage.ObjectRef{
		Provider:   pkgstorage.Provider(r.StorageProvider),
		BucketName: r.BucketName,
		ObjectKey:  r.ObjectKey,
	}
}

func scanCleanupRecords(rows *sql.Rows) ([]cleanupRecord, error) {
	defer rows.Close()

	var records []cleanupRecord
	for rows.Next() {
		var (
			record    cleanupRecord
			deletedAt sql.NullTime
		)
		if err := rows.Scan(
			&record.FileID,
			&record.BlobObjectID,
			&record.StorageProvider,
			&record.BucketName,
			&record.ObjectKey,
			&record.ReferenceCount,
			&deletedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	return limit
}

func rowsAffected(result sql.Result) (int, error) {
	if result == nil {
		return 0, nil
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

var (
	_ ports.UploadSessionRepository = (*UploadSessionRepository)(nil)
	_ ports.FileCleanupRepository   = (*FileCleanupRepository)(nil)
	_ ports.BlobRepository          = (*BlobRepository)(nil)
	_ ports.MultipartRepository     = (*MultipartRepository)(nil)
	_ ports.TenantUsageRepository   = (*TenantUsageRepository)(nil)
	_ ports.OutboxReader            = (*OutboxReader)(nil)
)
