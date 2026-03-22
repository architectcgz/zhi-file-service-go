CREATE TABLE file.blob_objects (
  blob_object_id VARCHAR(26) PRIMARY KEY,
  tenant_id VARCHAR(32) NOT NULL,
  storage_provider VARCHAR(32) NOT NULL,
  bucket_name VARCHAR(128) NOT NULL,
  object_key VARCHAR(512) NOT NULL,
  etag VARCHAR(255),
  checksum VARCHAR(255),
  hash_value VARCHAR(128),
  hash_algorithm VARCHAR(16),
  file_size BIGINT NOT NULL,
  content_type VARCHAR(255),
  reference_count INTEGER NOT NULL DEFAULT 0,
  storage_class VARCHAR(32),
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  deleted_at TIMESTAMPTZ,
  CONSTRAINT ck_blob_objects_storage_provider CHECK (storage_provider IN ('MINIO', 'S3')),
  CONSTRAINT ck_blob_objects_hash_algorithm CHECK (
    hash_algorithm IS NULL OR hash_algorithm IN ('MD5', 'SHA256')
  ),
  CONSTRAINT ck_blob_objects_hash_pair CHECK (
    (hash_value IS NULL AND hash_algorithm IS NULL) OR
    (hash_value IS NOT NULL AND hash_algorithm IS NOT NULL)
  ),
  CONSTRAINT ck_blob_objects_file_size_non_negative CHECK (file_size >= 0),
  CONSTRAINT ck_blob_objects_reference_count_non_negative CHECK (reference_count >= 0),
  CONSTRAINT uq_blob_objects_object_locator UNIQUE (tenant_id, storage_provider, bucket_name, object_key),
  CONSTRAINT uq_blob_objects_hash UNIQUE (tenant_id, hash_algorithm, hash_value, bucket_name)
);

CREATE INDEX idx_blob_objects_ref_count ON file.blob_objects (reference_count);
CREATE INDEX idx_blob_objects_deleted_at ON file.blob_objects (deleted_at);
