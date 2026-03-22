package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/queries"
	adminpostgres "github.com/architectcgz/zhi-file-service-go/internal/services/admin/infra/postgres"
	httptransport "github.com/architectcgz/zhi-file-service-go/internal/services/admin/transport/http"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
)

const defaultDevToken = "dev-token"

func Build(app *bootstrap.App) (bootstrap.RuntimeOptions, error) {
	if app == nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("bootstrap app is nil")
	}
	if app.DB == nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("bootstrap database is nil")
	}

	idgen := ids.NewGenerator(nil, nil)
	clk := clock.SystemClock{}

	tenants := adminpostgres.NewTenantRepository(app.DB)
	policies := adminpostgres.NewTenantPolicyRepository(app.DB)
	usages := adminpostgres.NewTenantUsageRepository(app.DB)
	files := adminpostgres.NewAdminFileRepository(app.DB)
	audits := adminpostgres.NewAuditLogRepository(app.DB)
	outbox := adminpostgres.NewOutboxPublisher(app.DB, idgen)
	txm := adminpostgres.NewTxManager(app.DB)

	createTenant := commands.NewCreateTenantHandler(tenants, policies, usages, audits, txm, idgen, clk)
	patchTenant := commands.NewPatchTenantHandler(tenants, audits, txm, idgen, clk)
	patchTenantPolicy := commands.NewPatchTenantPolicyHandler(policies, audits, txm, idgen, clk)
	deleteFile := commands.NewDeleteFileHandler(files, audits, outbox, txm, idgen, clk)

	listConfig := queries.ListTenantsConfig{
		ListDefaultLimit: app.Config.Admin.ListDefaultLimit,
		ListMaxLimit:     app.Config.Admin.ListMaxLimit,
	}
	fileListConfig := queries.ListFilesConfig{
		ListDefaultLimit: app.Config.Admin.ListDefaultLimit,
		ListMaxLimit:     app.Config.Admin.ListMaxLimit,
	}
	auditListConfig := queries.ListAuditLogsConfig{
		ListDefaultLimit: app.Config.Admin.ListDefaultLimit,
		ListMaxLimit:     app.Config.Admin.ListMaxLimit,
	}

	handler := httptransport.NewHandler(httptransport.Options{
		Auth:              httptransport.NewDevelopmentAuthResolver(developmentAuthConfig()),
		CreateTenant:      createTenant,
		ListTenants:       queries.NewListTenantsHandler(tenants, listConfig),
		GetTenant:         queries.NewGetTenantHandler(tenants),
		PatchTenant:       patchTenant,
		GetTenantPolicy:   queries.NewGetTenantPolicyHandler(policies),
		PatchTenantPolicy: patchTenantPolicy,
		GetTenantUsage:    queries.NewGetTenantUsageHandler(usages),
		ListFiles:         queries.NewListFilesHandler(files, fileListConfig),
		GetFile:           queries.NewGetFileHandler(files),
		DeleteFile:        deleteFile,
		ListAuditLogs:     queries.NewListAuditLogsHandler(audits, auditListConfig),
	})

	return bootstrap.RuntimeOptions{
		Handler: handler,
		Ready: func(ctx context.Context, app *bootstrap.App) error {
			if app == nil || app.DB == nil {
				return fmt.Errorf("admin runtime dependencies are not ready")
			}
			return ensureAdminTables(ctx, app.DB)
		},
	}, nil
}

func developmentAuthConfig() httptransport.DevelopmentAuthConfig {
	return httptransport.DevelopmentAuthConfig{
		Token:        firstNonEmpty(os.Getenv("ADMIN_DEV_TOKEN"), defaultDevToken),
		AdminID:      firstNonEmpty(os.Getenv("ADMIN_DEV_ADMIN_ID"), "admin-dev"),
		Roles:        splitCSV(os.Getenv("ADMIN_DEV_ROLES")),
		TenantScopes: splitCSV(os.Getenv("ADMIN_DEV_TENANT_SCOPES")),
		Permissions:  splitCSV(os.Getenv("ADMIN_DEV_PERMISSIONS")),
		TokenID:      strings.TrimSpace(os.Getenv("ADMIN_DEV_TOKEN_ID")),
	}
}

func ensureAdminTables(ctx context.Context, db *sql.DB) error {
	required := [][2]string{
		{"tenant", "tenants"},
		{"tenant", "tenant_policies"},
		{"tenant", "tenant_usage"},
		{"file", "file_assets"},
		{"audit", "admin_audit_logs"},
		{"infra", "outbox_events"},
	}
	for _, table := range required {
		var exists bool
		if err := db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM information_schema.tables
  WHERE table_schema = $1 AND table_name = $2
)`, table[0], table[1]).Scan(&exists); err != nil {
			return fmt.Errorf("check table %s.%s: %w", table[0], table[1], err)
		}
		if !exists {
			return fmt.Errorf("required table %s.%s is not available", table[0], table[1])
		}
	}
	return nil
}

func splitCSV(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
