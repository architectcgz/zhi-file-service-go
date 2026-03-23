package runtime

import (
	"context"
	"fmt"

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
	authResolver, err := httptransport.NewJWKSAuthResolverWithIssuers(app.Config.Access.AuthJWKS, app.Config.Access.AuthAllowedIssuers)
	if err != nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("build access auth resolver: %w", err)
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
	metricsRecorder := httptransport.NewMetricsRecorder(app.Metrics.Registry(), app.Config.App.ServiceName)

	handler := httptransport.NewHandler(httptransport.Options{
		Auth:                   authResolver,
		Metrics:                metricsRecorder,
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
