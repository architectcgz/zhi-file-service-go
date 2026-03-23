package postgres

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestTenantPolicyRepositoryGetByTenantIDDecodesTextArrays(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewTenantPolicyRepository(db)
	now := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"tenant_id",
		"max_storage_bytes",
		"max_file_count",
		"max_single_file_size",
		"allowed_mime_types",
		"allowed_extensions",
		"default_access_level",
		"auto_create_enabled",
		"created_at",
		"updated_at",
	}).AddRow(
		"demo",
		int64(10737418240),
		int64(100000),
		int64(104857600),
		"{image/png,application/pdf}",
		"{png,pdf}",
		"PRIVATE",
		true,
		now,
		now,
	)

	mock.ExpectQuery("SELECT tenant_id, max_storage_bytes, max_file_count, max_single_file_size, allowed_mime_types,").
		WithArgs("demo").
		WillReturnRows(rows)

	view, err := repo.GetByTenantID(context.Background(), "demo")
	if err != nil {
		t.Fatalf("GetByTenantID() error = %v", err)
	}
	if view == nil {
		t.Fatal("GetByTenantID() returned nil view")
	}
	if got := view.Policy.AllowedMimeTypes; len(got) != 2 || got[0] != "image/png" || got[1] != "application/pdf" {
		t.Fatalf("AllowedMimeTypes = %#v, want [image/png application/pdf]", got)
	}
	if got := view.Policy.AllowedExtensions; len(got) != 2 || got[0] != "png" || got[1] != "pdf" {
		t.Fatalf("AllowedExtensions = %#v, want [png pdf]", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
