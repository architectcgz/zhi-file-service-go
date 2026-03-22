CREATE TABLE tenant.tenant_usage (
  tenant_id VARCHAR(32) PRIMARY KEY,
  used_storage_bytes BIGINT NOT NULL DEFAULT 0,
  used_file_count BIGINT NOT NULL DEFAULT 0,
  last_upload_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT fk_tenant_usage_tenant
    FOREIGN KEY (tenant_id) REFERENCES tenant.tenants (tenant_id) ON DELETE CASCADE,
  CONSTRAINT ck_tenant_usage_storage_non_negative CHECK (used_storage_bytes >= 0),
  CONSTRAINT ck_tenant_usage_count_non_negative CHECK (used_file_count >= 0),
  CONSTRAINT ck_tenant_usage_version_non_negative CHECK (version >= 0)
);

CREATE INDEX idx_tenant_usage_updated_at ON tenant.tenant_usage (updated_at DESC);
