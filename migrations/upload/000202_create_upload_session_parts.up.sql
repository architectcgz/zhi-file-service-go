CREATE TABLE upload.upload_session_parts (
  upload_session_id VARCHAR(26) NOT NULL,
  part_number INTEGER NOT NULL,
  etag VARCHAR(255) NOT NULL,
  part_size BIGINT NOT NULL,
  checksum VARCHAR(255),
  uploaded_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT pk_upload_session_parts PRIMARY KEY (upload_session_id, part_number),
  CONSTRAINT fk_upload_session_parts_session
    FOREIGN KEY (upload_session_id) REFERENCES upload.upload_sessions (upload_session_id) ON DELETE CASCADE,
  CONSTRAINT ck_upload_session_parts_number CHECK (part_number > 0),
  CONSTRAINT ck_upload_session_parts_size CHECK (part_size > 0)
);

CREATE INDEX idx_upload_session_parts_uploaded_at
  ON upload.upload_session_parts (uploaded_at DESC);
