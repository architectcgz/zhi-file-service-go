package commands

import (
	"context"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type AbortUploadSessionCommand struct {
	UploadSessionID string
	Reason          string
	IdempotencyKey  string
	Auth            domain.AuthContext
}

type AbortUploadSessionHandler struct {
	sessions  ports.SessionRepository
	multipart ports.MultipartManager
	clock     clock.Clock
}

func NewAbortUploadSessionHandler(
	sessions ports.SessionRepository,
	multipart ports.MultipartManager,
	clk clock.Clock,
) AbortUploadSessionHandler {
	if clk == nil {
		clk = clock.SystemClock{}
	}

	return AbortUploadSessionHandler{
		sessions:  sessions,
		multipart: multipart,
		clock:     clk,
	}
}

func (h AbortUploadSessionHandler) Handle(ctx context.Context, command AbortUploadSessionCommand) (view.UploadSession, error) {
	if err := command.Auth.RequireFileWrite(); err != nil {
		return view.UploadSession{}, err
	}
	if strings.TrimSpace(command.UploadSessionID) == "" {
		return view.UploadSession{}, xerrors.New(xerrors.CodeInvalidArgument, "upload session id is required", xerrors.Details{
			"field": "uploadSessionId",
		})
	}

	session, err := h.sessions.GetByID(ctx, command.Auth.TenantID, command.UploadSessionID)
	if err != nil {
		return view.UploadSession{}, err
	}
	if err := command.Auth.EnsureSessionAccess(session); err != nil {
		return view.UploadSession{}, err
	}

	if err := session.Abort(h.clock.Now()); err != nil {
		return view.UploadSession{}, err
	}
	if err := h.sessions.Save(ctx, session); err != nil {
		return view.UploadSession{}, err
	}

	if session.Mode == domain.SessionModeDirect && session.ProviderUploadID != "" && h.multipart != nil {
		_ = h.multipart.AbortMultipartUpload(ctx, session.Object, session.ProviderUploadID)
	}

	return view.FromSession(session, "", nil), nil
}
