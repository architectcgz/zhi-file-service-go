package ports

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
)

type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	GetByID(ctx context.Context, tenantID string, uploadSessionID string) (*domain.Session, error)
	FindReusable(ctx context.Context, tenantID string, fingerprint string) (*domain.Session, error)
	Save(ctx context.Context, session *domain.Session) error
}
