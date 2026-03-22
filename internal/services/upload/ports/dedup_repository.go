package ports

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
)

type DedupRepository interface {
	LookupByHash(ctx context.Context, tenantID string, hash domain.ContentHash) (*domain.DedupDecision, error)
	ClaimHash(ctx context.Context, tenantID string, uploadSessionID string, hash domain.ContentHash) error
}
