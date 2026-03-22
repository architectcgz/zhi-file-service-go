CREATE TABLE upload.upload_sessions (
  upload_session_id VARCHAR(26) PRIMARY KEY,
  tenant_id VARCHAR(32) NOT NULL,
  owner_id VARCHAR(64) NOT NULL,
  upload_mode VARCHAR(32) NOT NULL,
  target_access_level VARCHAR(16) NOT NULL,
  original_filename VARCHAR(255) NOT NULL,
  content_type VARCHAR(255),
  expected_size BIGINT NOT NULL,
  file_hash VARCHAR(128),
  hash_algorithm VARCHAR(16),
  storage_provider VARCHAR(32) NOT NULL,
  bucket_name VARCHAR(128) NOT NULL,
  object_key VARCHAR(512),
  provider_upload_id VARCHAR(255),
  chunk_size_bytes INTEGER NOT NULL DEFAULT 0,
  total_parts INTEGER NOT NULL DEFAULT 1,
  completed_parts INTEGER NOT NULL DEFAULT 0,
  file_id VARCHAR(26),
  completion_token VARCHAR(128),
  completion_started_at TIMESTAMPTZ,
  status VARCHAR(16) NOT NULL,
  failure_code VARCHAR(64),
  failure_message VARCHAR(500),
  resumed_from_session_id VARCHAR(26),
  idempotency_key VARCHAR(128),
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  aborted_at TIMESTAMPTZ,
  failed_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT ck_upload_sessions_mode CHECK (upload_mode IN ('INLINE', 'DIRECT', 'PRESIGNED_SINGLE')),
  CONSTRAINT ck_upload_sessions_target_access CHECK (target_access_level IN ('PUBLIC', 'PRIVATE')),
  CONSTRAINT ck_upload_sessions_expected_size CHECK (expected_size > 0),
  CONSTRAINT ck_upload_sessions_hash_algorithm CHECK (
    hash_algorithm IS NULL OR hash_algorithm IN ('MD5', 'SHA256')
  ),
  CONSTRAINT ck_upload_sessions_chunk_size CHECK (chunk_size_bytes >= 0),
  CONSTRAINT ck_upload_sessions_parts_min CHECK (total_parts >= 1),
  CONSTRAINT ck_upload_sessions_completed_parts CHECK (
    completed_parts >= 0 AND completed_parts <= total_parts
  ),
  CONSTRAINT ck_upload_sessions_status CHECK (
    status IN ('INITIATED', 'UPLOADING', 'COMPLETING', 'COMPLETED', 'ABORTED', 'EXPIRED', 'FAILED')
  )
);

CREATE INDEX idx_upload_sessions_owner_active
  ON upload.upload_sessions (tenant_id, owner_id, status, created_at DESC);

CREATE INDEX idx_upload_sessions_expires_at
  ON upload.upload_sessions (expires_at)
  WHERE status IN ('INITIATED', 'UPLOADING', 'COMPLETING');

CREATE INDEX idx_upload_sessions_file_hash
  ON upload.upload_sessions (tenant_id, owner_id, file_hash, expected_size, status);

CREATE INDEX idx_upload_sessions_file_id
  ON upload.upload_sessions (file_id);
