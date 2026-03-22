package bootstrap_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		next := filepath.Dir(wd)
		if next == wd {
			t.Fatalf("repo root not found from %s", wd)
		}
		wd = next
	}
}

func runScript(t *testing.T, scriptPath string, args []string, env map[string]string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestMigrateBuildBuildsFlatView(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/bootstrap/migrate-build.sh")
	fixtureRoot := filepath.Join(repoRoot, "test/fixtures/migrate-build/valid")
	buildRoot := filepath.Join(t.TempDir(), "all")

	out, err := runScript(t, scriptPath, nil, map[string]string{
		"MIGRATIONS_ROOT":   fixtureRoot,
		"MIGRATE_BUILD_DIR": buildRoot,
	})
	if err != nil {
		t.Fatalf("migrate-build failed: %v\noutput:\n%s", err, out)
	}

	expectedFiles := []string{
		"000001_create_tenant_schema.down.sql",
		"000001_create_tenant_schema.up.sql",
		"000010_create_file_schema.down.sql",
		"000010_create_file_schema.up.sql",
	}
	for _, name := range expectedFiles {
		if _, err := os.Stat(filepath.Join(buildRoot, name)); err != nil {
			t.Fatalf("expected built migration %s: %v", name, err)
		}
	}
}

func TestMigrateBuildFailsOnDuplicateVersion(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/bootstrap/migrate-build.sh")
	fixtureRoot := filepath.Join(repoRoot, "test/fixtures/migrate-build/duplicate-version")
	buildRoot := filepath.Join(t.TempDir(), "all")

	out, err := runScript(t, scriptPath, nil, map[string]string{
		"MIGRATIONS_ROOT":   fixtureRoot,
		"MIGRATE_BUILD_DIR": buildRoot,
	})
	if err == nil {
		t.Fatalf("expected duplicate version failure, output:\n%s", out)
	}
	if !strings.Contains(out, "重复") && !strings.Contains(strings.ToLower(out), "duplicate") {
		t.Fatalf("expected duplicate hint, output:\n%s", out)
	}
}

func TestMigrateBuildFailsOnMissingPair(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/bootstrap/migrate-build.sh")
	fixtureRoot := filepath.Join(repoRoot, "test/fixtures/migrate-build/missing-pair")
	buildRoot := filepath.Join(t.TempDir(), "all")

	out, err := runScript(t, scriptPath, nil, map[string]string{
		"MIGRATIONS_ROOT":   fixtureRoot,
		"MIGRATE_BUILD_DIR": buildRoot,
	})
	if err == nil {
		t.Fatalf("expected missing pair failure, output:\n%s", out)
	}
	if !strings.Contains(out, "成对") && !strings.Contains(strings.ToLower(out), "pair") {
		t.Fatalf("expected pair hint, output:\n%s", out)
	}
}

func TestSeedScriptReadsSeedDirectoryInOrder(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/bootstrap/seed.sh")
	seedRoot := filepath.Join(repoRoot, "test/fixtures/seed")

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "psql.log")
	mockPSQL := filepath.Join(tmpDir, "mock-psql.sh")
	mockScript := "#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" >> \"${LOG_FILE}\"\n"
	if err := os.WriteFile(mockPSQL, []byte(mockScript), 0o755); err != nil {
		t.Fatalf("write mock psql: %v", err)
	}

	out, err := runScript(t, scriptPath, []string{"dev"}, map[string]string{
		"SEED_ROOT": seedRoot,
		"PSQL_BIN":  mockPSQL,
		"LOG_FILE":  logFile,
		"DB_DSN":    "postgres://example/test",
	})
	if err != nil {
		t.Fatalf("seed script failed: %v\noutput:\n%s", err, out)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read psql log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 seed files executed, got %d, logs=%q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "000001_tenant.sql") || !strings.Contains(lines[1], "000010_usage.sql") {
		t.Fatalf("seed files order mismatch, logs=%q", lines)
	}
}

func TestSeedScriptRejectsProd(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/bootstrap/seed.sh")
	seedRoot := filepath.Join(repoRoot, "test/fixtures/seed")

	out, err := runScript(t, scriptPath, []string{"prod"}, map[string]string{
		"SEED_ROOT": seedRoot,
	})
	if err == nil {
		t.Fatalf("expected prod to be rejected, output:\n%s", out)
	}
	if !strings.Contains(out, "禁止") {
		t.Fatalf("expected rejection message, output:\n%s", out)
	}
}
