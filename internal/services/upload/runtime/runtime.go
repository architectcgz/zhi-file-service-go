package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	uploadpostgres "github.com/architectcgz/zhi-file-service-go/internal/services/upload/infra/postgres"
	storageinfra "github.com/architectcgz/zhi-file-service-go/internal/services/upload/infra/storage"
	httptransport "github.com/architectcgz/zhi-file-service-go/internal/services/upload/transport/http"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
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
	authResolver, err := httptransport.NewJWKSAuthResolverWithIssuers(app.Config.Upload.AuthJWKS, app.Config.Upload.AuthAllowedIssuers)
	if err != nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("build upload auth resolver: %w", err)
	}

	storageAdapter, err := storageinfra.NewAdapter(app.Storage, app.Config.Storage)
	if err != nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("build upload storage adapter: %w", err)
	}

	idgen := ids.NewGenerator(nil, nil)
	clk := clock.SystemClock{}
	sessions := uploadpostgres.NewSessionRepository(app.DB)
	parts := uploadpostgres.NewSessionPartRepository(app.DB)
	blobs := uploadpostgres.NewBlobRepository(app.DB)
	files := uploadpostgres.NewFileRepository(app.DB)
	dedup := uploadpostgres.NewDedupRepository(app.DB)
	policies := uploadpostgres.NewTenantPolicyReader(app.DB)
	usage := uploadpostgres.NewTenantUsageRepository(app.DB)
	outbox := uploadpostgres.NewOutboxPublisher(app.DB, idgen)
	txm := uploadpostgres.NewTxManager(app.DB)

	createUploadSession := commands.NewCreateUploadSessionHandler(
		sessions,
		policies,
		storageAdapter,
		storageAdapter,
		storageAdapter,
		idgen,
		clk,
		commands.CreateUploadSessionConfig{
			SessionTTL:   app.Config.Upload.SessionTTL,
			PresignTTL:   app.Config.Upload.PresignTTL,
			AllowedModes: allowedModes(app.Config.Upload.AllowedModes),
		},
	)
	getUploadSession := queries.NewGetUploadSessionHandler(sessions)
	uploadInlineContent := commands.NewUploadInlineContentHandler(sessions, storageAdapter, clk)
	presignMultipartParts := commands.NewPresignMultipartPartsHandler(
		sessions,
		storageAdapter,
		clk,
		commands.PresignMultipartPartsConfig{
			DefaultTTL: app.Config.Upload.PresignTTL,
			MaxTTL:     maxDuration(app.Config.Upload.SessionTTL, 24*time.Hour),
		},
	)
	listUploadedParts := queries.NewListUploadedPartsHandler(sessions, parts, storageAdapter, clk)
	completeUploadSession := commands.NewCompleteUploadSessionHandler(
		sessions,
		parts,
		blobs,
		files,
		dedup,
		usage,
		outbox,
		txm,
		storageAdapter,
		storageAdapter,
		idgen,
		clk,
	)
	abortUploadSession := commands.NewAbortUploadSessionHandler(sessions, storageAdapter, clk)
	metricsRecorder := httptransport.NewMetricsRecorder(app.Metrics.Registry(), app.Config.App.ServiceName)

	handler := httptransport.NewHandler(httptransport.Options{
		Auth:                  authResolver,
		Metrics:               metricsRecorder,
		MaxInlineBodyBytes:    app.Config.Upload.MaxInlineSize,
		CreateUploadSession:   createUploadSession,
		GetUploadSession:      getUploadSession,
		UploadInlineContent:   uploadInlineContent,
		PresignMultipartParts: presignMultipartParts,
		ListUploadedParts:     listUploadedParts,
		CompleteUploadSession: completeUploadSession,
		AbortUploadSession:    abortUploadSession,
	})

	return bootstrap.RuntimeOptions{
		Handler: handler,
		Ready: func(_ context.Context, app *bootstrap.App) error {
			if app == nil || app.DB == nil || app.Storage == nil {
				return fmt.Errorf("upload runtime dependencies are not ready")
			}
			return nil
		},
	}, nil
}

func allowedModes(values []string) []domain.SessionMode {
	if len(values) == 0 {
		return nil
	}

	result := make([]domain.SessionMode, 0, len(values))
	for _, value := range values {
		mode := domain.SessionMode(strings.ToUpper(strings.TrimSpace(value)))
		if mode == "" {
			continue
		}
		result = append(result, mode)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func maxDuration(left time.Duration, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}
