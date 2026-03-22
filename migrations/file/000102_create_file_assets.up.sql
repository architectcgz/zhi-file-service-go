CREATE TABLE file.file_assets (
  file_id VARCHAR(26) PRIMARY KEY,
  tenant_id VARCHAR(32) NOT NULL,
  owner_id VARCHAR(64) NOT NULL,
  blob_object_id VARCHAR(26) NOT NULL,
  original_filename VARCHAR(255) NOT NULL,
  content_type VARCHAR(255),
  file_size BIGINT NOT NULL,
  access_level VARCHAR(16) NOT NULL,
  status VARCHAR(16) NOT NULL,
  storage_provider VARCHAR(32) NOT NULL,
  bucket_name VARCHAR(128) NOT NULL,
  object_key VARCHAR(512) NOT NULL,
  file_hash VARCHAR(128),
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  deleted_at TIMESTAMPTZ,
  CONSTRAINT fk_file_assets_blob_object
    FOREIGN KEY (blob_object_id) REFERENCES file.blob_objects (blob_object_id),
  CONSTRAINT ck_file_assets_size_non_negative CHECK (file_size >= 0),
  CONSTRAINT ck_file_assets_access_level CHECK (access_level IN ('PUBLIC', 'PRIVATE')),
  CONSTRAINT ck_file_assets_status CHECK (status IN ('ACTIVE', 'DELETED'))
);

CREATE INDEX idx_file_assets_tenant_owner ON file.file_assets (tenant_id, owner_id, created_at DESC);
CREATE INDEX idx_file_assets_tenant_status ON file.file_assets (tenant_id, status, created_at DESC);
CREATE INDEX idx_file_assets_blob_object ON file.file_assets (blob_object_id);
CREATE INDEX idx_file_assets_object_locator ON file.file_assets (tenant_id, bucket_name, object_key);
