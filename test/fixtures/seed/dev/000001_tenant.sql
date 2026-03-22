INSERT INTO tenant.tenants (tenant_id, tenant_name, status, created_at, updated_at)
VALUES ('demo', 'Demo Tenant', 'ACTIVE', NOW(), NOW())
ON CONFLICT (tenant_id) DO UPDATE SET updated_at = EXCLUDED.updated_at;
