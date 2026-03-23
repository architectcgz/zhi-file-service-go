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

func writeMockScript(t *testing.T, dir string, name string, body string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write mock script %s: %v", name, err)
	}
	return path
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
	if !strings.Contains(out, "成对") &&
		!strings.Contains(strings.ToLower(out), "pair") &&
		!strings.Contains(strings.ToLower(out), "both up and down") {
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

func TestE2EScriptRunsGoTestOnE2EPackage(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/test/e2e.sh")

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "go.log")
	mockGo := writeMockScript(t, tmpDir, "mock-go.sh", "#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" >> \"${LOG_FILE}\"\n")

	out, err := runScript(t, scriptPath, nil, map[string]string{
		"GO_BIN":   mockGo,
		"LOG_FILE": logFile,
	})
	if err != nil {
		t.Fatalf("e2e script failed: %v\noutput:\n%s", err, out)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read go log: %v", err)
	}
	got := strings.TrimSpace(string(data))
	want := "test -count=1 -timeout=300s ./test/e2e/..."
	if got != want {
		t.Fatalf("go args = %q, want %q", got, want)
	}
}

func TestPerformanceScriptRunsBenchmarksByDefault(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/test/performance.sh")

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "go.log")
	mockGo := writeMockScript(t, tmpDir, "mock-go.sh", "#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" >> \"${LOG_FILE}\"\n")

	out, err := runScript(t, scriptPath, nil, map[string]string{
		"GO_BIN":   mockGo,
		"LOG_FILE": logFile,
	})
	if err != nil {
		t.Fatalf("performance script failed: %v\noutput:\n%s", err, out)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read go log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 benchmark commands, got %d, logs=%q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "-bench Benchmark(CreateUploadSessionInline|CompleteUploadSessionPresignedSingle)") {
		t.Fatalf("unexpected upload benchmark command: %q", lines[0])
	}
	if !strings.Contains(lines[1], "-bench Benchmark(GetFilePublic|ResolveDownloadPrivate|RedirectByAccessTicketPrivate)") {
		t.Fatalf("unexpected access benchmark command: %q", lines[1])
	}
}

func TestPerformanceScriptRunsK6ScenarioWhenRequested(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/test/performance.sh")

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "k6.log")
	mockK6 := writeMockScript(t, tmpDir, "mock-k6.sh", "#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" >> \"${LOG_FILE}\"\n")

	out, err := runScript(t, scriptPath, nil, map[string]string{
		"K6_BIN":       mockK6,
		"LOG_FILE":     logFile,
		"PERF_MODE":    "k6",
		"PERF_TARGET":  "access",
		"BASE_URL":     "http://127.0.0.1:8081",
		"BEARER_TOKEN": "dev-token",
		"FILE_ID":      "file-1",
	})
	if err != nil {
		t.Fatalf("performance script failed: %v\noutput:\n%s", err, out)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read k6 log: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if !strings.Contains(got, "run") || !strings.Contains(got, "test/performance/access-read-hotpath.js") {
		t.Fatalf("unexpected k6 command: %q", got)
	}
}

func TestDoctorScriptRequiresUploadAuthJWKS(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/tools/doctor.sh")

	env := baseDoctorEnv("upload-service")
	delete(env, "UPLOAD_AUTH_JWKS")

	out, err := runScript(t, scriptPath, nil, env)
	if err == nil {
		t.Fatalf("expected doctor to fail, output:\n%s", out)
	}
	if !strings.Contains(out, "UPLOAD_AUTH_JWKS") {
		t.Fatalf("expected UPLOAD_AUTH_JWKS missing hint, output:\n%s", out)
	}
}

func TestDoctorScriptRequiresAccessAuthJWKS(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts/tools/doctor.sh")

	env := baseDoctorEnv("access-service")
	delete(env, "ACCESS_AUTH_JWKS")

	out, err := runScript(t, scriptPath, nil, env)
	if err == nil {
		t.Fatalf("expected doctor to fail, output:\n%s", out)
	}
	if !strings.Contains(out, "ACCESS_AUTH_JWKS") {
		t.Fatalf("expected ACCESS_AUTH_JWKS missing hint, output:\n%s", out)
	}
}

func baseDoctorEnv(serviceName string) map[string]string {
	env := map[string]string{
		"APP_ENV":                   "test",
		"APP_SERVICE_NAME":          serviceName,
		"DB_DSN":                    "postgres://test:test@127.0.0.1:5432/test",
		"STORAGE_ENDPOINT":          "http://127.0.0.1:9000",
		"STORAGE_ACCESS_KEY":        "key",
		"STORAGE_SECRET_KEY":        "secret",
		"STORAGE_PUBLIC_BUCKET":     "public",
		"STORAGE_PRIVATE_BUCKET":    "private",
		"REDIS_ADDR":                "127.0.0.1:6379",
		"UPLOAD_AUTH_JWKS":          "inline-jwks",
		"ACCESS_TICKET_SIGNING_KEY": "ticket-key",
		"ACCESS_AUTH_JWKS":          "inline-jwks",
		"ADMIN_AUTH_JWKS":           "inline-jwks",
	}
	return env
}
