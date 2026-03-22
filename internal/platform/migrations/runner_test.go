package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectSortsByVersionAscending(t *testing.T) {
	t.Parallel()

	sourceRoot := t.TempDir()

	mustWriteFile(t, filepath.Join(sourceRoot, "file", "000100_create_file_schema.up.sql"), "-- up")
	mustWriteFile(t, filepath.Join(sourceRoot, "file", "000100_create_file_schema.down.sql"), "-- down")
	mustWriteFile(t, filepath.Join(sourceRoot, "tenant", "000001_create_tenant_schema.up.sql"), "-- up")
	mustWriteFile(t, filepath.Join(sourceRoot, "tenant", "000001_create_tenant_schema.down.sql"), "-- down")
	mustWriteFile(t, filepath.Join(sourceRoot, "tenant", "000002_create_tenants.up.sql"), "-- up")
	mustWriteFile(t, filepath.Join(sourceRoot, "tenant", "000002_create_tenants.down.sql"), "-- down")

	migrations, err := Collect(sourceRoot)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if len(migrations) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(migrations))
	}

	got := []int64{migrations[0].Version, migrations[1].Version, migrations[2].Version}
	want := []int64{1, 2, 100}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected sorted versions %v, got %v", want, got)
		}
	}
}

func TestCollectRejectsDuplicateVersionAcrossSchemas(t *testing.T) {
	t.Parallel()

	sourceRoot := t.TempDir()

	mustWriteFile(t, filepath.Join(sourceRoot, "tenant", "000001_create_tenant_schema.up.sql"), "-- up")
	mustWriteFile(t, filepath.Join(sourceRoot, "tenant", "000001_create_tenant_schema.down.sql"), "-- down")
	mustWriteFile(t, filepath.Join(sourceRoot, "file", "000001_create_file_schema.up.sql"), "-- up")
	mustWriteFile(t, filepath.Join(sourceRoot, "file", "000001_create_file_schema.down.sql"), "-- down")

	_, err := Collect(sourceRoot)
	if err == nil {
		t.Fatalf("expected duplicate version error, got nil")
	}

	if !strings.Contains(err.Error(), "duplicate version 000001") {
		t.Fatalf("expected duplicate version error, got %v", err)
	}
}

func TestBuildLinearViewCopiesFlattenedSQLFiles(t *testing.T) {
	t.Parallel()

	sourceRoot := t.TempDir()
	outputRoot := filepath.Join(t.TempDir(), "all")

	mustWriteFile(t, filepath.Join(sourceRoot, "tenant", "000001_create_tenant_schema.up.sql"), "CREATE SCHEMA tenant;")
	mustWriteFile(t, filepath.Join(sourceRoot, "tenant", "000001_create_tenant_schema.down.sql"), "DROP SCHEMA tenant;")
	mustWriteFile(t, filepath.Join(sourceRoot, "file", "000100_create_file_schema.up.sql"), "CREATE SCHEMA file;")
	mustWriteFile(t, filepath.Join(sourceRoot, "file", "000100_create_file_schema.down.sql"), "DROP SCHEMA file;")

	migrations, err := BuildLinearView(sourceRoot, outputRoot)
	if err != nil {
		t.Fatalf("BuildLinearView returned error: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	entries, err := os.ReadDir(outputRoot)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}

	if len(entries) != 4 {
		t.Fatalf("expected 4 flattened files, got %d", len(entries))
	}

	if entries[0].Name() != "000001_create_tenant_schema.down.sql" {
		t.Fatalf("unexpected first file %q", entries[0].Name())
	}
	if entries[1].Name() != "000001_create_tenant_schema.up.sql" {
		t.Fatalf("unexpected second file %q", entries[1].Name())
	}
	if entries[2].Name() != "000100_create_file_schema.down.sql" {
		t.Fatalf("unexpected third file %q", entries[2].Name())
	}
	if entries[3].Name() != "000100_create_file_schema.up.sql" {
		t.Fatalf("unexpected fourth file %q", entries[3].Name())
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", path, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", path, err)
	}
}
