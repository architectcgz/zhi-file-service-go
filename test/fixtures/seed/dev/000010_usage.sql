INSERT INTO tenant.tenant_usage (tenant_id, used_storage_bytes, used_file_count, version, updated_at)
VALUES ('demo', 0, 0, 0, NOW())
ON CONFLICT (tenant_id) DO UPDATE SET updated_at = EXCLUDED.updated_at;
