CREATE TABLE upload.upload_dedup_claims (
  tenant_id VARCHAR(32) NOT NULL,
  hash_algorithm VARCHAR(16) NOT NULL,
  hash_value VARCHAR(128) NOT NULL,
  bucket_name VARCHAR(128) NOT NULL,
  owner_token VARCHAR(64) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT pk_upload_dedup_claims PRIMARY KEY (tenant_id, hash_algorithm, hash_value, bucket_name),
  CONSTRAINT ck_upload_dedup_claims_hash_algorithm CHECK (hash_algorithm IN ('MD5', 'SHA256'))
);

CREATE INDEX idx_upload_dedup_claims_expires_at
  ON upload.upload_dedup_claims (expires_at);
