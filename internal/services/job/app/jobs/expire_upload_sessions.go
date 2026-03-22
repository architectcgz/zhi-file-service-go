package jobs

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

const JobNameExpireUploadSessions = "expire_upload_sessions"

type ExpireUploadSessionsConfig struct {
	BatchSize   int
	ExpireAfter time.Duration
}

type ExpireUploadSessionsJob struct {
	repo        ports.UploadSessionRepository
	clock       clock.Clock
	batchSize   int
	expireAfter time.Duration
}

func NewExpireUploadSessionsJob(
	repo ports.UploadSessionRepository,
	clk clock.Clock,
	cfg ExpireUploadSessionsConfig,
) ExpireUploadSessionsJob {
	if clk == nil {
		clk = clock.SystemClock{}
	}
	if cfg.ExpireAfter <= 0 {
		cfg.ExpireAfter = 30 * time.Minute
	}

	return ExpireUploadSessionsJob{
		repo:        repo,
		clock:       clk,
		batchSize:   normalizeBatchSize(cfg.BatchSize),
		expireAfter: cfg.ExpireAfter,
	}
}

func (j ExpireUploadSessionsJob) Name() string {
	return JobNameExpireUploadSessions
}

func (j ExpireUploadSessionsJob) Execute(ctx context.Context) error {
	_, err := j.repo.ExpirePendingSessions(ctx, j.clock.Now().Add(-j.expireAfter), j.batchSize)
	return err
}
