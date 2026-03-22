INSERT INTO audit.admin_audit_logs (
  audit_log_id,
  admin_subject,
  action,
  target_type,
  target_id,
  tenant_id,
  request_id,
  ip_address,
  user_agent,
  details,
  created_at
) VALUES (
  '01HSEEDAUDIT00000000000000',
  'seed:system',
  'TENANT_BOOTSTRAP',
  'TENANT',
  'demo',
  'demo',
  'seed-dev-req-001',
  '127.0.0.1',
  'seed-script',
  '{"source": "bootstrap/seed/dev", "note": "minimal sample"}'::jsonb,
  NOW()
)
ON CONFLICT (audit_log_id) DO NOTHING;

INSERT INTO infra.outbox_events (
  event_id,
  service_name,
  aggregate_type,
  aggregate_id,
  event_type,
  payload,
  status,
  retry_count,
  next_attempt_at,
  published_at,
  created_at,
  updated_at
) VALUES (
  '01HSEEDEVENT00000000000000',
  'admin-service',
  'TENANT',
  'demo',
  'tenant.seeded.v1',
  '{"tenant_id": "demo", "seed_env": "dev"}'::jsonb,
  'PENDING',
  0,
  NOW(),
  NULL,
  NOW(),
  NOW()
)
ON CONFLICT (event_id) DO UPDATE
SET payload = EXCLUDED.payload,
    status = EXCLUDED.status,
    retry_count = EXCLUDED.retry_count,
    next_attempt_at = EXCLUDED.next_attempt_at,
    updated_at = NOW();
