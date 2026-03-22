CREATE TABLE tenant.tenant_policies (
  tenant_id VARCHAR(32) PRIMARY KEY,
  max_storage_bytes BIGINT NOT NULL,
  max_file_count BIGINT NOT NULL,
  max_single_file_size BIGINT NOT NULL,
  allowed_mime_types TEXT[],
  allowed_extensions TEXT[],
  default_access_level VARCHAR(16) NOT NULL,
  auto_create_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT fk_tenant_policies_tenant
    FOREIGN KEY (tenant_id) REFERENCES tenant.tenants (tenant_id) ON DELETE CASCADE,
  CONSTRAINT ck_tenant_policies_storage_bytes CHECK (max_storage_bytes > 0),
  CONSTRAINT ck_tenant_policies_file_count CHECK (max_file_count >= 0),
  CONSTRAINT ck_tenant_policies_single_file_size CHECK (max_single_file_size > 0),
  CONSTRAINT ck_tenant_policies_access_level CHECK (default_access_level IN ('PUBLIC', 'PRIVATE'))
);
