package postgres

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

func TestUploadSessionRepositoryExpirePendingSessions(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewUploadSessionRepository(db)
	cutoff := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec("WITH candidates AS").
		WithArgs(cutoff, 50).
		WillReturnResult(sqlmock.NewResult(0, 3))

	processed, err := repo.ExpirePendingSessions(context.Background(), cutoff, 50)
	if err != nil {
		t.Fatalf("ExpirePendingSessions() error = %v", err)
	}
	if processed != 3 {
		t.Fatalf("processed = %d, want 3", processed)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestFileCleanupRepositoryFinalizeDeletedFilesDeletesObjectAndMarksBlob(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	storage := &stubObjectStorage{}
	repo := NewFileCleanupRepository(db, storage)
	eligibleBefore := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"file_id", "blob_object_id", "storage_provider", "bucket_name", "object_key", "reference_count", "blob_deleted_at",
	}).AddRow("file-1", "blob-1", "S3", "private-bucket", "tenant-a/file-1", 0, nil)

	mock.ExpectBegin()
	mock.ExpectQuery("WITH candidates AS").
		WithArgs(eligibleBefore, 10).
		WillReturnRows(rows)
	mock.ExpectExec("UPDATE file.blob_objects").
		WithArgs(sqlmock.AnyArg(), "blob-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	processed, err := repo.FinalizeDeletedFiles(context.Background(), eligibleBefore, 10)
	if err != nil {
		t.Fatalf("FinalizeDeletedFiles() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if len(storage.deleted) != 1 {
		t.Fatalf("deleted objects = %#v, want 1", storage.deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestOutboxReaderClaimPendingDecodesPayload(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewOutboxReader(db)
	now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	payload := mustJSONValue(t, map[string]any{"fileId": "file-1"})
	nextAttemptAt := now.Add(time.Minute)

	rows := sqlmock.NewRows([]string{
		"event_id", "service_name", "aggregate_type", "aggregate_id", "event_type", "payload", "retry_count", "next_attempt_at",
	}).AddRow("evt-1", "admin-service", "file_asset", "file-1", "file.asset.delete_requested.v1", payload, 0, nextAttemptAt)

	mock.ExpectQuery("WITH candidates AS").
		WithArgs(now, "file.asset.delete_requested.v1", sqlmock.AnyArg(), 5).
		WillReturnRows(rows)

	events, err := repo.ClaimPending(context.Background(), ports.ClaimOutboxEventsQuery{
		EventTypes: []string{"file.asset.delete_requested.v1"},
		DueBefore:  now,
		Limit:      5,
	})
	if err != nil {
		t.Fatalf("ClaimPending() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Payload["fileId"] != "file-1" {
		t.Fatalf("payload = %#v, want fileId=file-1", events[0].Payload)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

type stubObjectStorage struct {
	deleted []pkgstorage.ObjectRef
}

func (s *stubObjectStorage) DeleteObject(_ context.Context, ref pkgstorage.ObjectRef) error {
	s.deleted = append(s.deleted, ref)
	return nil
}

func (s *stubObjectStorage) AbortMultipartUpload(context.Context, pkgstorage.ObjectRef, string) error {
	return nil
}

func mustJSONValue(t *testing.T, value map[string]any) driver.Value {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}
