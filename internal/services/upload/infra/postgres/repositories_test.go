package postgres

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestTenantPolicyReaderReadUploadPolicyDecodesTextArray(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewTenantPolicyReader(db)
	rows := sqlmock.NewRows([]string{"max_single_file_size", "allowed_mime_types"}).
		AddRow(int64(104857600), "{image/png,application/pdf}")

	mock.ExpectQuery("SELECT max_single_file_size, allowed_mime_types").
		WithArgs("demo").
		WillReturnRows(rows)

	policy, err := repo.ReadUploadPolicy(context.Background(), "demo")
	if err != nil {
		t.Fatalf("ReadUploadPolicy() error = %v", err)
	}
	if policy.MaxInlineSize != 104857600 {
		t.Fatalf("MaxInlineSize = %d, want 104857600", policy.MaxInlineSize)
	}
	if len(policy.AllowedMimeTypes) != 2 {
		t.Fatalf("AllowedMimeTypes len = %d, want 2", len(policy.AllowedMimeTypes))
	}
	if policy.AllowedMimeTypes[0] != "image/png" || policy.AllowedMimeTypes[1] != "application/pdf" {
		t.Fatalf("AllowedMimeTypes = %#v, want [image/png application/pdf]", policy.AllowedMimeTypes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
