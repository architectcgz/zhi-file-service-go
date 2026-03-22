package ports

import (
	"context"
	"time"
)

type MultipartRepository interface {
	CleanupMultipartUploads(ctx context.Context, staleBefore time.Time, limit int) (int, error)
}
