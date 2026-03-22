package ports

import (
	"context"
	"time"
)

type FileCleanupRepository interface {
	FinalizeDeletedFiles(ctx context.Context, eligibleBefore time.Time, limit int) (int, error)
}
