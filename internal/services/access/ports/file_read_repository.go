package ports

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
)

type FileReadRepository interface {
	GetByID(ctx context.Context, fileID string) (domain.FileView, error)
}
