CREATE TABLE audit.admin_audit_logs (
  audit_log_id VARCHAR(26) PRIMARY KEY,
  admin_subject VARCHAR(128) NOT NULL,
  action VARCHAR(64) NOT NULL,
  target_type VARCHAR(64) NOT NULL,
  target_id VARCHAR(64),
  tenant_id VARCHAR(32),
  request_id VARCHAR(64),
  ip_address VARCHAR(64),
  user_agent VARCHAR(255),
  details JSONB,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_admin_audit_logs_admin
  ON audit.admin_audit_logs (admin_subject, created_at DESC);

CREATE INDEX idx_admin_audit_logs_action
  ON audit.admin_audit_logs (action, created_at DESC);

CREATE INDEX idx_admin_audit_logs_tenant
  ON audit.admin_audit_logs (tenant_id, created_at DESC);

CREATE INDEX idx_admin_audit_logs_target
  ON audit.admin_audit_logs (target_type, target_id, created_at DESC);
