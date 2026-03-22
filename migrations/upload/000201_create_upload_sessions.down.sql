DROP INDEX IF EXISTS upload.idx_upload_sessions_file_id;
DROP INDEX IF EXISTS upload.idx_upload_sessions_file_hash;
DROP INDEX IF EXISTS upload.idx_upload_sessions_expires_at;
DROP INDEX IF EXISTS upload.idx_upload_sessions_owner_active;
DROP TABLE IF EXISTS upload.upload_sessions;
