package ports

import (
	"context"
	"time"
)

type BlobRepository interface {
	CleanupOrphanBlobs(ctx context.Context, staleBefore time.Time, limit int) (int, error)
}
