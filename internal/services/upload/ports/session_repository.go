package ports

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type ReusableSessionQuery struct {
	TenantID    string
	OwnerID     string
	Mode        domain.SessionMode
	AccessLevel pkgstorage.AccessLevel
	SizeBytes   int64
	Hash        *domain.ContentHash
}

type CompletionAcquireRequest struct {
	TenantID        string
	UploadSessionID string
	CompletionToken string
	StartedAt       time.Time
}

type CompletionAcquireResult struct {
	Session   *domain.Session
	Ownership domain.CompletionOwnership
}

type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	GetByID(ctx context.Context, tenantID string, uploadSessionID string) (*domain.Session, error)
	FindReusable(ctx context.Context, query ReusableSessionQuery) (*domain.Session, error)
	AcquireCompletion(ctx context.Context, request CompletionAcquireRequest) (*CompletionAcquireResult, error)
	ConfirmCompletionOwner(ctx context.Context, tenantID string, uploadSessionID string, completionToken string) (*domain.Session, error)
}
