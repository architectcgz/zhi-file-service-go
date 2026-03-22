package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/queries"
	accessidempotency "github.com/architectcgz/zhi-file-service-go/internal/services/access/infra/idempotency"
	accesspostgres "github.com/architectcgz/zhi-file-service-go/internal/services/access/infra/postgres"
	storageinfra "github.com/architectcgz/zhi-file-service-go/internal/services/access/infra/storage"
	tokenissuer "github.com/architectcgz/zhi-file-service-go/internal/services/access/infra/token"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
	httptransport "github.com/architectcgz/zhi-file-service-go/internal/services/access/transport/http"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

const (
	defaultDevToken       = "dev-token"
	defaultDevTenantID    = "demo"
	defaultDevSubjectID   = "dev-user"
	defaultDevSubjectType = "USER"
)

func Build(app *bootstrap.App) (bootstrap.RuntimeOptions, error) {
	if app == nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("bootstrap app is nil")
	}
	if app.DB == nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("bootstrap database is nil")
	}
	if app.Storage == nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("bootstrap storage is nil")
	}

	fileRepo := accesspostgres.NewFileReadRepository(app.DB)
	policies := accesspostgres.NewTenantPolicyReader(app.DB)
	storageAdapter, err := storageinfra.NewAdapter(app.Storage, app.Config.Storage)
	if err != nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("build access storage adapter: %w", err)
	}
	issuer, err := tokenissuer.NewHMACTicketIssuer(app.Config.Access.TicketSigningKey)
	if err != nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("build access ticket issuer: %w", err)
	}

	clk := clock.SystemClock{}
	var idempotencyStore ports.AccessTicketIdempotencyStore = accessidempotency.NewMemoryStore(clk)
	if app.Redis != nil && app.Redis.Raw() != nil {
		idempotencyStore = accessidempotency.NewRedisStore(app.Redis.Raw())
	} else if app.Logger != nil {
		app.Logger.Warn("access_ticket_idempotency_memory_fallback")
	}
	getFile := queries.NewGetFileHandler(fileRepo, storageAdapter, app.Config.Access.PublicURLEnabled)
	createTicket := commands.NewCreateAccessTicketHandler(
		fileRepo,
		policies,
		issuer,
		idempotencyStore,
		clk,
		app.Config.Access.TicketTTL,
		"/api/v1/access-tickets",
	)
	resolveDownload := queries.NewResolveDownloadHandler(
		fileRepo,
		policies,
		storageAdapter,
		storageAdapter,
		app.Config.Access.PrivatePresignTTL,
		app.Config.Access.PublicURLEnabled,
	)
	redirectByTicket := queries.NewRedirectByAccessTicketHandler(
		fileRepo,
		policies,
		issuer,
		storageAdapter,
		storageAdapter,
		clk,
		app.Config.Access.PrivatePresignTTL,
		app.Config.Access.PublicURLEnabled,
	)

	handler := httptransport.NewHandler(httptransport.Options{
		Auth:                   httptransport.NewDevelopmentAuthResolver(developmentAuthConfig()),
		GetFile:                getFile,
		CreateAccessTicket:     createTicket,
		ResolveDownload:        resolveDownload,
		RedirectByAccessTicket: redirectByTicket,
	})

	return bootstrap.RuntimeOptions{
		Handler: handler,
		Ready: func(_ context.Context, app *bootstrap.App) error {
			if app == nil || app.DB == nil || app.Storage == nil {
				return fmt.Errorf("access runtime dependencies are not ready")
			}
			return nil
		},
	}, nil
}

func developmentAuthConfig() httptransport.DevelopmentAuthConfig {
	return httptransport.DevelopmentAuthConfig{
		Token:       firstNonEmpty(os.Getenv("ACCESS_DEV_TOKEN"), defaultDevToken),
		TenantID:    firstNonEmpty(os.Getenv("ACCESS_DEV_TENANT_ID"), defaultDevTenantID),
		SubjectID:   firstNonEmpty(os.Getenv("ACCESS_DEV_SUBJECT_ID"), defaultDevSubjectID),
		SubjectType: firstNonEmpty(os.Getenv("ACCESS_DEV_SUBJECT_TYPE"), defaultDevSubjectType),
		ClientID:    strings.TrimSpace(os.Getenv("ACCESS_DEV_CLIENT_ID")),
		TokenID:     strings.TrimSpace(os.Getenv("ACCESS_DEV_TOKEN_ID")),
		Scopes:      splitCSV(os.Getenv("ACCESS_DEV_SCOPES")),
	}
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
