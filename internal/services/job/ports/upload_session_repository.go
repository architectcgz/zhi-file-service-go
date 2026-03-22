package ports

import (
	"context"
	"time"
)

type UploadSessionRepository interface {
	ExpirePendingSessions(ctx context.Context, expiredBefore time.Time, limit int) (int, error)
	RepairStuckCompleting(ctx context.Context, staleBefore time.Time, limit int) (int, error)
}
