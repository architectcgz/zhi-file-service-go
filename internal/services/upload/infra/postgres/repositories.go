package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type SessionRepository struct {
	db *sql.DB
}

type SessionPartRepository struct {
	db *sql.DB
}

type BlobRepository struct {
	db *sql.DB
}

type FileRepository struct {
	db *sql.DB
}

type DedupRepository struct {
	db *sql.DB
}

type TenantPolicyReader struct {
	db *sql.DB
}

type TenantUsageRepository struct {
	db *sql.DB
}

type OutboxPublisher struct {
	db    *sql.DB
	idgen ids.Generator
}

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type txKey struct{}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func NewSessionPartRepository(db *sql.DB) *SessionPartRepository {
	return &SessionPartRepository{db: db}
}

func NewBlobRepository(db *sql.DB) *BlobRepository {
	return &BlobRepository{db: db}
}

func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

func NewDedupRepository(db *sql.DB) *DedupRepository {
	return &DedupRepository{db: db}
}

func NewTenantPolicyReader(db *sql.DB) *TenantPolicyReader {
	return &TenantPolicyReader{db: db}
}

func NewTenantUsageRepository(db *sql.DB) *TenantUsageRepository {
	return &TenantUsageRepository{db: db}
}

func NewOutboxPublisher(db *sql.DB, idgen ids.Generator) *OutboxPublisher {
	if idgen == nil {
		idgen = ids.NewGenerator(nil, nil)
	}
	return &OutboxPublisher{db: db, idgen: idgen}
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	if session == nil {
		return xerrors.New(xerrors.CodeInvalidArgument, "upload session is required", nil)
	}
	query := `
INSERT INTO upload.upload_sessions (
  upload_session_id, tenant_id, owner_id, upload_mode, target_access_level, original_filename,
  content_type, expected_size, file_hash, hash_algorithm, storage_provider, bucket_name,
  object_key, provider_upload_id, chunk_size_bytes, total_parts, completed_parts, file_id,
  completion_token, completion_started_at, status, failure_code, failure_message,
  resumed_from_session_id, idempotency_key, created_at, updated_at, completed_at,
  aborted_at, failed_at, expires_at
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, $9, $10, $11, $12,
  $13, $14, $15, $16, $17, $18,
  $19, $20, $21, $22, $23,
  $24, $25, $26, $27, $28,
  $29, $30, $31
)`
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, query,
		session.ID,
		session.TenantID,
		session.OwnerID,
		string(session.Mode),
		string(session.AccessLevel),
		session.FileName,
		nullString(session.ContentType),
		session.SizeBytes,
		hashValue(session.Hash),
		hashAlgorithm(session.Hash),
		string(session.Object.Provider),
		session.Object.BucketName,
		session.Object.ObjectKey,
		nullString(session.ProviderUploadID),
		session.ChunkSizeBytes,
		session.TotalParts,
		session.CompletedParts,
		nullString(session.FileID),
		nullString(session.CompletionToken),
		nullTime(session.CompletionStartedAt),
		string(session.Status),
		nullString(session.FailureCode),
		nullString(session.FailureMessage),
		nullString(session.ResumedFromSessionID),
		nullString(session.IdempotencyKey),
		session.CreatedAt.UTC(),
		session.UpdatedAt.UTC(),
		nullTime(session.CompletedAt),
		nullTime(session.AbortedAt),
		nullTime(session.FailedAt),
		session.ExpiresAt.UTC(),
	)
	if err != nil {
		return wrapDBError("create upload session", err)
	}
	return nil
}

func (r *SessionRepository) Save(ctx context.Context, session *domain.Session) error {
	if session == nil {
		return xerrors.New(xerrors.CodeInvalidArgument, "upload session is required", nil)
	}
	query := `
UPDATE upload.upload_sessions
SET owner_id = $3,
    upload_mode = $4,
    target_access_level = $5,
    original_filename = $6,
    content_type = $7,
    expected_size = $8,
    file_hash = $9,
    hash_algorithm = $10,
    storage_provider = $11,
    bucket_name = $12,
    object_key = $13,
    provider_upload_id = $14,
    chunk_size_bytes = $15,
    total_parts = $16,
    completed_parts = $17,
    file_id = $18,
    completion_token = $19,
    completion_started_at = $20,
    status = $21,
    failure_code = $22,
    failure_message = $23,
    resumed_from_session_id = $24,
    idempotency_key = $25,
    updated_at = $26,
    completed_at = $27,
    aborted_at = $28,
    failed_at = $29,
    expires_at = $30
WHERE tenant_id = $1 AND upload_session_id = $2`
	result, err := currentExecutor(ctx, r.db).ExecContext(ctx, query,
		session.TenantID,
		session.ID,
		session.OwnerID,
		string(session.Mode),
		string(session.AccessLevel),
		session.FileName,
		nullString(session.ContentType),
		session.SizeBytes,
		hashValue(session.Hash),
		hashAlgorithm(session.Hash),
		string(session.Object.Provider),
		session.Object.BucketName,
		session.Object.ObjectKey,
		nullString(session.ProviderUploadID),
		session.ChunkSizeBytes,
		session.TotalParts,
		session.CompletedParts,
		nullString(session.FileID),
		nullString(session.CompletionToken),
		nullTime(session.CompletionStartedAt),
		string(session.Status),
		nullString(session.FailureCode),
		nullString(session.FailureMessage),
		nullString(session.ResumedFromSessionID),
		nullString(session.IdempotencyKey),
		session.UpdatedAt.UTC(),
		nullTime(session.CompletedAt),
		nullTime(session.AbortedAt),
		nullTime(session.FailedAt),
		session.ExpiresAt.UTC(),
	)
	if err != nil {
		return wrapDBError("save upload session", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		return domain.ErrUploadSessionNotFound(session.ID)
	}
	return nil
}

func (r *SessionRepository) GetByID(ctx context.Context, tenantID string, uploadSessionID string) (*domain.Session, error) {
	session, err := querySession(ctx, currentExecutor(ctx, r.db), `
SELECT upload_session_id, tenant_id, owner_id, upload_mode, target_access_level, original_filename,
       content_type, expected_size, file_hash, hash_algorithm, storage_provider, bucket_name,
       object_key, provider_upload_id, chunk_size_bytes, total_parts, completed_parts, file_id,
       completion_token, completion_started_at, status, failure_code, failure_message,
       resumed_from_session_id, idempotency_key, created_at, updated_at, completed_at,
       aborted_at, failed_at, expires_at
FROM upload.upload_sessions
WHERE tenant_id = $1 AND upload_session_id = $2`, tenantID, uploadSessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUploadSessionNotFound(uploadSessionID)
		}
		return nil, wrapDBError("get upload session", err)
	}
	return session, nil
}

func (r *SessionRepository) FindReusable(ctx context.Context, query ports.ReusableSessionQuery) (*domain.Session, error) {
	session, err := querySession(ctx, currentExecutor(ctx, r.db), `
SELECT upload_session_id, tenant_id, owner_id, upload_mode, target_access_level, original_filename,
       content_type, expected_size, file_hash, hash_algorithm, storage_provider, bucket_name,
       object_key, provider_upload_id, chunk_size_bytes, total_parts, completed_parts, file_id,
       completion_token, completion_started_at, status, failure_code, failure_message,
       resumed_from_session_id, idempotency_key, created_at, updated_at, completed_at,
       aborted_at, failed_at, expires_at
FROM upload.upload_sessions
WHERE tenant_id = $1
  AND owner_id = $2
  AND upload_mode = $3
  AND target_access_level = $4
  AND expected_size = $5
  AND status IN ('INITIATED', 'UPLOADING')
  AND (
    ($6 = FALSE AND file_hash IS NULL AND hash_algorithm IS NULL) OR
    ($6 = TRUE AND file_hash = $7 AND hash_algorithm = $8)
  )
ORDER BY created_at DESC
LIMIT 1`,
		query.TenantID,
		query.OwnerID,
		string(query.Mode),
		string(query.AccessLevel),
		query.SizeBytes,
		query.Hash != nil,
		hashValue(query.Hash),
		hashAlgorithm(query.Hash),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDBError("find reusable upload session", err)
	}
	return session, nil
}

func (r *SessionRepository) AcquireCompletion(ctx context.Context, request ports.CompletionAcquireRequest) (*ports.CompletionAcquireResult, error) {
	if currentTx(ctx) != nil {
		return r.acquireCompletion(ctx, currentTx(ctx), request)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapDBError("begin acquire completion transaction", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := r.acquireCompletion(withTx(ctx, tx), tx, request)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, wrapDBError("commit acquire completion transaction", err)
	}
	return result, nil
}

func (r *SessionRepository) acquireCompletion(ctx context.Context, exec executor, request ports.CompletionAcquireRequest) (*ports.CompletionAcquireResult, error) {
	session, err := querySession(ctx, exec, `
SELECT upload_session_id, tenant_id, owner_id, upload_mode, target_access_level, original_filename,
       content_type, expected_size, file_hash, hash_algorithm, storage_provider, bucket_name,
       object_key, provider_upload_id, chunk_size_bytes, total_parts, completed_parts, file_id,
       completion_token, completion_started_at, status, failure_code, failure_message,
       resumed_from_session_id, idempotency_key, created_at, updated_at, completed_at,
       aborted_at, failed_at, expires_at
FROM upload.upload_sessions
WHERE tenant_id = $1 AND upload_session_id = $2
FOR UPDATE`,
		request.TenantID,
		request.UploadSessionID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUploadSessionNotFound(request.UploadSessionID)
		}
		return nil, wrapDBError("acquire completion", err)
	}

	ownership, err := session.AcquireCompletion(request.CompletionToken, request.StartedAt)
	if err != nil {
		return nil, err
	}
	if ownership == domain.CompletionOwnershipAcquired {
		if err := r.Save(ctx, session); err != nil {
			return nil, err
		}
	}
	return &ports.CompletionAcquireResult{
		Session:   session,
		Ownership: ownership,
	}, nil
}

func (r *SessionRepository) ConfirmCompletionOwner(ctx context.Context, tenantID string, uploadSessionID string, completionToken string) (*domain.Session, error) {
	session, err := querySession(ctx, currentExecutor(ctx, r.db), `
SELECT upload_session_id, tenant_id, owner_id, upload_mode, target_access_level, original_filename,
       content_type, expected_size, file_hash, hash_algorithm, storage_provider, bucket_name,
       object_key, provider_upload_id, chunk_size_bytes, total_parts, completed_parts, file_id,
       completion_token, completion_started_at, status, failure_code, failure_message,
       resumed_from_session_id, idempotency_key, created_at, updated_at, completed_at,
       aborted_at, failed_at, expires_at
FROM upload.upload_sessions
WHERE tenant_id = $1 AND upload_session_id = $2 AND completion_token = $3
FOR UPDATE`,
		tenantID, uploadSessionID, completionToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUploadSessionNotFound(uploadSessionID)
		}
		return nil, wrapDBError("confirm completion owner", err)
	}
	return session, nil
}

func (r *SessionPartRepository) ListBySessionID(ctx context.Context, tenantID string, uploadSessionID string) ([]ports.SessionPartRecord, error) {
	rows, err := currentExecutor(ctx, r.db).QueryContext(ctx, `
SELECT p.upload_session_id, p.part_number, p.etag, p.part_size, p.checksum, p.uploaded_at
FROM upload.upload_session_parts p
JOIN upload.upload_sessions s ON s.upload_session_id = p.upload_session_id
WHERE s.tenant_id = $1 AND p.upload_session_id = $2
ORDER BY p.part_number ASC`, tenantID, uploadSessionID)
	if err != nil {
		return nil, wrapDBError("list upload session parts", err)
	}
	defer rows.Close()

	result := make([]ports.SessionPartRecord, 0)
	for rows.Next() {
		var record ports.SessionPartRecord
		var checksum sql.NullString
		if err := rows.Scan(&record.UploadSessionID, &record.PartNumber, &record.ETag, &record.PartSize, &checksum, &record.UploadedAt); err != nil {
			return nil, wrapDBError("scan upload session part", err)
		}
		record.Checksum = checksum.String
		result = append(result, record)
	}
	return result, wrapDBError("iterate upload session parts", rows.Err())
}

func (r *SessionPartRepository) Upsert(ctx context.Context, record ports.SessionPartRecord) error {
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
INSERT INTO upload.upload_session_parts (upload_session_id, part_number, etag, part_size, checksum, uploaded_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (upload_session_id, part_number)
DO UPDATE SET etag = EXCLUDED.etag,
              part_size = EXCLUDED.part_size,
              checksum = EXCLUDED.checksum,
              uploaded_at = EXCLUDED.uploaded_at`,
		record.UploadSessionID,
		record.PartNumber,
		record.ETag,
		record.PartSize,
		nullString(record.Checksum),
		record.UploadedAt.UTC(),
	)
	if err != nil {
		return wrapDBError("upsert upload session part", err)
	}
	return nil
}

func (r *SessionPartRepository) Replace(ctx context.Context, tenantID string, uploadSessionID string, parts []ports.SessionPartRecord) error {
	if _, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
DELETE FROM upload.upload_session_parts p
USING upload.upload_sessions s
WHERE s.upload_session_id = p.upload_session_id
  AND s.tenant_id = $1
  AND p.upload_session_id = $2`, tenantID, uploadSessionID); err != nil {
		return wrapDBError("delete upload session parts", err)
	}
	for _, part := range parts {
		if err := r.Upsert(ctx, part); err != nil {
			return err
		}
	}
	return nil
}

func (r *BlobRepository) Upsert(ctx context.Context, record ports.BlobRecord) error {
	now := time.Now().UTC()
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
INSERT INTO file.blob_objects (
  blob_object_id, tenant_id, storage_provider, bucket_name, object_key, etag, checksum,
  hash_value, hash_algorithm, file_size, content_type, reference_count, created_at, updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11, 0, $12, $13
)
ON CONFLICT (blob_object_id)
DO UPDATE SET storage_provider = EXCLUDED.storage_provider,
              bucket_name = EXCLUDED.bucket_name,
              object_key = EXCLUDED.object_key,
              etag = EXCLUDED.etag,
              checksum = EXCLUDED.checksum,
              hash_value = EXCLUDED.hash_value,
              hash_algorithm = EXCLUDED.hash_algorithm,
              file_size = EXCLUDED.file_size,
              content_type = EXCLUDED.content_type,
              updated_at = EXCLUDED.updated_at`,
		record.BlobID,
		record.TenantID,
		string(record.StorageProvider),
		record.BucketName,
		record.ObjectKey,
		nullString(record.ETag),
		nullString(record.Checksum),
		hashValueValue(record.Hash),
		hashAlgorithmValue(record.Hash),
		record.SizeBytes,
		nullString(record.ContentType),
		now,
		now,
	)
	if err != nil {
		return wrapDBError("upsert blob object", err)
	}
	return nil
}

func (r *BlobRepository) AdjustReferenceCount(ctx context.Context, blobID string, delta int64) error {
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
UPDATE file.blob_objects
SET reference_count = reference_count + $2,
    updated_at = $3
WHERE blob_object_id = $1`, blobID, delta, time.Now().UTC())
	if err != nil {
		return wrapDBError("adjust blob reference count", err)
	}
	return nil
}

func (r *FileRepository) CreateFileAsset(ctx context.Context, record ports.FileAssetRecord) error {
	now := time.Now().UTC()
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
INSERT INTO file.file_assets (
  file_id, tenant_id, owner_id, blob_object_id, original_filename, content_type,
  file_size, access_level, status, storage_provider, bucket_name, object_key,
  file_hash, metadata, created_at, updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, 'ACTIVE', $9, $10, $11,
  $12, NULL, $13, $14
)`,
		record.FileID,
		record.TenantID,
		record.OwnerID,
		record.BlobID,
		record.FileName,
		nullString(record.ContentType),
		record.SizeBytes,
		string(record.AccessLevel),
		string(record.StorageProvider),
		record.BucketName,
		record.ObjectKey,
		hashValue(record.Hash),
		now,
		now,
	)
	if err != nil {
		return wrapDBError("create file asset", err)
	}
	return nil
}

func (r *DedupRepository) LookupByHash(ctx context.Context, key ports.DedupLookupKey) (*domain.DedupDecision, error) {
	row := currentExecutor(ctx, r.db).QueryRowContext(ctx, `
SELECT b.blob_object_id, f.file_id, COALESCE(b.etag, ''), b.file_size
FROM file.blob_objects b
JOIN file.file_assets f ON f.blob_object_id = b.blob_object_id
WHERE b.tenant_id = $1
  AND b.bucket_name = $2
  AND b.hash_algorithm = $3
  AND b.hash_value = $4
  AND b.deleted_at IS NULL
  AND f.status = 'ACTIVE'
ORDER BY f.created_at DESC
LIMIT 1`,
		key.TenantID,
		key.BucketName,
		key.Hash.Algorithm,
		key.Hash.Value,
	)
	var blobID, fileID, etag string
	var sizeBytes int64
	if err := row.Scan(&blobID, &fileID, &etag, &sizeBytes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDBError("lookup dedup hash", err)
	}
	return &domain.DedupDecision{
		Hit:            true,
		BlobID:         blobID,
		FileID:         fileID,
		CanonicalETag:  etag,
		CanonicalBytes: sizeBytes,
	}, nil
}

func (r *DedupRepository) ClaimHash(ctx context.Context, claim ports.DedupClaim) error {
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
INSERT INTO upload.upload_dedup_claims (
  tenant_id, hash_algorithm, hash_value, bucket_name, owner_token, expires_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (tenant_id, hash_algorithm, hash_value, bucket_name)
DO UPDATE SET owner_token = EXCLUDED.owner_token,
              expires_at = EXCLUDED.expires_at,
              updated_at = EXCLUDED.updated_at`,
		claim.TenantID,
		claim.Hash.Algorithm,
		claim.Hash.Value,
		claim.BucketName,
		claim.OwnerToken,
		claim.ExpiresAt.UTC(),
		time.Now().UTC(),
		time.Now().UTC(),
	)
	if err != nil {
		return wrapDBError("claim dedup hash", err)
	}
	return nil
}

func (r *TenantPolicyReader) ReadUploadPolicy(ctx context.Context, tenantID string) (ports.TenantUploadPolicy, error) {
	row := currentExecutor(ctx, r.db).QueryRowContext(ctx, `
SELECT max_single_file_size, allowed_mime_types
FROM tenant.tenant_policies
WHERE tenant_id = $1`, tenantID)
	var maxSingleFileSize int64
	var allowedMimeTypes []string
	if err := row.Scan(&maxSingleFileSize, &allowedMimeTypes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.TenantUploadPolicy{}, xerrors.New(xerrors.CodeForbidden, "tenant policy is not configured", xerrors.Details{
				"tenantId": tenantID,
			})
		}
		return ports.TenantUploadPolicy{}, wrapDBError("read tenant upload policy", err)
	}
	return ports.TenantUploadPolicy{
		AllowInlineUpload: true,
		AllowMultipart:    true,
		MaxInlineSize:     maxSingleFileSize,
		MaxFileSize:       maxSingleFileSize,
		AllowedMimeTypes:  allowedMimeTypes,
	}, nil
}

func (r *TenantUsageRepository) ApplyDelta(ctx context.Context, tenantID string, deltaBytes int64) error {
	_, err := currentExecutor(ctx, r.db).ExecContext(ctx, `
UPDATE tenant.tenant_usage
SET used_storage_bytes = used_storage_bytes + $2,
    last_upload_at = CASE WHEN $2 > 0 THEN $3 ELSE last_upload_at END,
    version = version + 1,
    updated_at = $3
WHERE tenant_id = $1`,
		tenantID,
		deltaBytes,
		time.Now().UTC(),
	)
	if err != nil {
		return wrapDBError("apply tenant usage delta", err)
	}
	return nil
}

func (p *OutboxPublisher) Enqueue(ctx context.Context, message ports.OutboxMessage) error {
	eventID, err := p.idgen.New()
	if err != nil {
		return xerrors.Wrap(xerrors.CodeInternalError, "generate outbox event id", err, nil)
	}
	now := time.Now().UTC()
	_, err = currentExecutor(ctx, p.db).ExecContext(ctx, `
INSERT INTO infra.outbox_events (
  event_id, service_name, aggregate_type, aggregate_id, event_type, payload,
  status, retry_count, next_attempt_at, published_at, created_at, updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6,
  'PENDING', 0, NULL, NULL, $7, $8
)`,
		eventID,
		"upload-service",
		"upload_session",
		message.AggregateID,
		message.EventType,
		message.Payload,
		now,
		now,
	)
	if err != nil {
		return wrapDBError("enqueue outbox event", err)
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

func querySession(ctx context.Context, exec executor, query string, args ...any) (*domain.Session, error) {
	row := exec.QueryRowContext(ctx, query, args...)
	var (
		uploadSessionID      string
		tenantID             string
		ownerID              string
		uploadMode           string
		targetAccessLevel    string
		originalFilename     string
		contentType          sql.NullString
		expectedSize         int64
		fileHash             sql.NullString
		hashAlgorithm        sql.NullString
		storageProvider      string
		bucketName           string
		objectKey            string
		providerUploadID     sql.NullString
		chunkSizeBytes       int
		totalParts           int
		completedParts       int
		fileID               sql.NullString
		completionToken      sql.NullString
		completionStartedAt  sql.NullTime
		status               string
		failureCode          sql.NullString
		failureMessage       sql.NullString
		resumedFromSessionID sql.NullString
		idempotencyKey       sql.NullString
		createdAt            time.Time
		updatedAt            time.Time
		completedAt          sql.NullTime
		abortedAt            sql.NullTime
		failedAt             sql.NullTime
		expiresAt            time.Time
	)
	if err := row.Scan(
		&uploadSessionID,
		&tenantID,
		&ownerID,
		&uploadMode,
		&targetAccessLevel,
		&originalFilename,
		&contentType,
		&expectedSize,
		&fileHash,
		&hashAlgorithm,
		&storageProvider,
		&bucketName,
		&objectKey,
		&providerUploadID,
		&chunkSizeBytes,
		&totalParts,
		&completedParts,
		&fileID,
		&completionToken,
		&completionStartedAt,
		&status,
		&failureCode,
		&failureMessage,
		&resumedFromSessionID,
		&idempotencyKey,
		&createdAt,
		&updatedAt,
		&completedAt,
		&abortedAt,
		&failedAt,
		&expiresAt,
	); err != nil {
		return nil, err
	}

	var hash *domain.ContentHash
	if fileHash.Valid && hashAlgorithm.Valid {
		hash = &domain.ContentHash{
			Algorithm: hashAlgorithm.String,
			Value:     fileHash.String,
		}
	}

	return domain.NewSession(domain.CreateSessionParams{
		ID:             uploadSessionID,
		TenantID:       tenantID,
		OwnerID:        ownerID,
		FileName:       originalFilename,
		ContentType:    contentType.String,
		SizeBytes:      expectedSize,
		AccessLevel:    pkgstorage.AccessLevel(targetAccessLevel),
		Mode:           domain.SessionMode(uploadMode),
		Status:         domain.SessionStatus(status),
		ChunkSizeBytes: chunkSizeBytes,
		TotalParts:     totalParts,
		CompletedParts: completedParts,
		Object: pkgstorage.ObjectRef{
			Provider:   pkgstorage.Provider(storageProvider),
			BucketName: bucketName,
			ObjectKey:  objectKey,
		},
		ProviderUploadID:     providerUploadID.String,
		FileID:               fileID.String,
		Hash:                 hash,
		CompletionToken:      completionToken.String,
		CompletionStartedAt:  timePtr(completionStartedAt),
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
		CompletedAt:          timePtr(completedAt),
		AbortedAt:            timePtr(abortedAt),
		FailureCode:          failureCode.String,
		FailureMessage:       failureMessage.String,
		FailedAt:             timePtr(failedAt),
		ResumedFromSessionID: resumedFromSessionID.String,
		IdempotencyKey:       idempotencyKey.String,
		ExpiresAt:            expiresAt,
	})
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed := value.Time.UTC()
	return &parsed
}

func hashValue(hash *domain.ContentHash) any {
	if hash == nil {
		return nil
	}
	return hash.Value
}

func hashAlgorithm(hash *domain.ContentHash) any {
	if hash == nil {
		return nil
	}
	return hash.Algorithm
}

func hashValueValue(hash domain.ContentHash) any {
	if hash.Algorithm == "" || hash.Value == "" {
		return nil
	}
	return hash.Value
}

func hashAlgorithmValue(hash domain.ContentHash) any {
	if hash.Algorithm == "" || hash.Value == "" {
		return nil
	}
	return hash.Algorithm
}

func wrapDBError(message string, err error) error {
	if err == nil {
		return nil
	}
	return xerrors.Wrap(xerrors.CodeServiceUnavailable, message, err, nil)
}

type TxManager struct {
	db *sql.DB
}

func NewTxManager(db *sql.DB) TxManager {
	return TxManager{db: db}
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

var (
	_ ports.SessionRepository     = (*SessionRepository)(nil)
	_ ports.SessionPartRepository = (*SessionPartRepository)(nil)
	_ ports.BlobRepository        = (*BlobRepository)(nil)
	_ ports.FileRepository        = (*FileRepository)(nil)
	_ ports.DedupRepository       = (*DedupRepository)(nil)
	_ ports.TenantPolicyReader    = (*TenantPolicyReader)(nil)
	_ ports.TenantUsageRepository = (*TenantUsageRepository)(nil)
	_ ports.OutboxPublisher       = (*OutboxPublisher)(nil)
)

func assertDB(db *sql.DB) *sql.DB {
	if db == nil {
		panic(fmt.Sprintf("%T requires a non-nil *sql.DB", db))
	}
	return db
}
