package ports

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
)

type DedupLookupKey struct {
	TenantID   string
	BucketName string
	Hash       domain.ContentHash
}

type DedupClaim struct {
	TenantID        string
	BucketName      string
	UploadSessionID string
	Hash            domain.ContentHash
	OwnerToken      string
	ExpiresAt       time.Time
}

type DedupRepository interface {
	LookupByHash(ctx context.Context, key DedupLookupKey) (*domain.DedupDecision, error)
	ClaimHash(ctx context.Context, claim DedupClaim) error
}
