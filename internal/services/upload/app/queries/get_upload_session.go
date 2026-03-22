package queries

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
)

type GetUploadSessionQuery struct {
	UploadSessionID string
	Auth            domain.AuthContext
}

type GetUploadSessionHandler struct {
	sessions ports.SessionRepository
}

func NewGetUploadSessionHandler(sessions ports.SessionRepository) GetUploadSessionHandler {
	return GetUploadSessionHandler{sessions: sessions}
}

func (h GetUploadSessionHandler) Handle(ctx context.Context, query GetUploadSessionQuery) (view.UploadSession, error) {
	if err := query.Auth.RequireFileWrite(); err != nil {
		return view.UploadSession{}, err
	}

	session, err := h.sessions.GetByID(ctx, query.Auth.TenantID, query.UploadSessionID)
	if err != nil {
		return view.UploadSession{}, err
	}
	if err := query.Auth.EnsureSessionAccess(session); err != nil {
		return view.UploadSession{}, err
	}

	return view.FromSession(session, "", nil), nil
}
