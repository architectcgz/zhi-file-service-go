CREATE TABLE tenant.tenants (
  tenant_id VARCHAR(32) PRIMARY KEY,
  tenant_name VARCHAR(128) NOT NULL,
  status VARCHAR(16) NOT NULL,
  contact_email VARCHAR(255),
  description VARCHAR(500),
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  deleted_at TIMESTAMPTZ,
  CONSTRAINT ck_tenants_status CHECK (status IN ('ACTIVE', 'SUSPENDED', 'DELETED'))
);

CREATE INDEX idx_tenants_status ON tenant.tenants (status);
CREATE INDEX idx_tenants_created_at ON tenant.tenants (created_at DESC);
