INSERT INTO tenant.tenants (
  tenant_id,
  tenant_name,
  status,
  contact_email,
  description,
  created_at,
  updated_at,
  deleted_at
) VALUES (
  'demo',
  'Demo Tenant',
  'ACTIVE',
  'demo@example.local',
  'bootstrap dev seed tenant',
  NOW(),
  NOW(),
  NULL
)
ON CONFLICT (tenant_id) DO UPDATE
SET tenant_name = EXCLUDED.tenant_name,
    status = EXCLUDED.status,
    contact_email = EXCLUDED.contact_email,
    description = EXCLUDED.description,
    updated_at = NOW();

INSERT INTO tenant.tenant_policies (
  tenant_id,
  max_storage_bytes,
  max_file_count,
  max_single_file_size,
  allowed_mime_types,
  allowed_extensions,
  default_access_level,
  auto_create_enabled,
  created_at,
  updated_at
) VALUES (
  'demo',
  10737418240,
  100000,
  104857600,
  ARRAY['image/png', 'image/jpeg', 'application/pdf'],
  ARRAY['png', 'jpg', 'jpeg', 'pdf'],
  'PRIVATE',
  FALSE,
  NOW(),
  NOW()
)
ON CONFLICT (tenant_id) DO UPDATE
SET max_storage_bytes = EXCLUDED.max_storage_bytes,
    max_file_count = EXCLUDED.max_file_count,
    max_single_file_size = EXCLUDED.max_single_file_size,
    allowed_mime_types = EXCLUDED.allowed_mime_types,
    allowed_extensions = EXCLUDED.allowed_extensions,
    default_access_level = EXCLUDED.default_access_level,
    auto_create_enabled = EXCLUDED.auto_create_enabled,
    updated_at = NOW();

INSERT INTO tenant.tenant_usage (
  tenant_id,
  used_storage_bytes,
  used_file_count,
  last_upload_at,
  version,
  updated_at
) VALUES (
  'demo',
  0,
  0,
  NULL,
  0,
  NOW()
)
ON CONFLICT (tenant_id) DO UPDATE
SET used_storage_bytes = EXCLUDED.used_storage_bytes,
    used_file_count = EXCLUDED.used_file_count,
    last_upload_at = EXCLUDED.last_upload_at,
    updated_at = NOW();
