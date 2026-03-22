package commands

import (
	"context"
	"io"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type UploadInlineContentCommand struct {
	UploadSessionID string
	ContentType     string
	Body            io.Reader
	Auth            domain.AuthContext
}

type UploadInlineContentHandler struct {
	sessions ports.SessionRepository
	writer   ports.InlineObjectWriter
	clock    clock.Clock
}

func NewUploadInlineContentHandler(
	sessions ports.SessionRepository,
	writer ports.InlineObjectWriter,
	clk clock.Clock,
) UploadInlineContentHandler {
	if clk == nil {
		clk = clock.SystemClock{}
	}

	return UploadInlineContentHandler{
		sessions: sessions,
		writer:   writer,
		clock:    clk,
	}
}

func (h UploadInlineContentHandler) Handle(ctx context.Context, command UploadInlineContentCommand) (view.UploadSession, error) {
	if err := command.Auth.RequireFileWrite(); err != nil {
		return view.UploadSession{}, err
	}
	if strings.TrimSpace(command.UploadSessionID) == "" {
		return view.UploadSession{}, xerrors.New(xerrors.CodeInvalidArgument, "upload session id is required", xerrors.Details{
			"field": "uploadSessionId",
		})
	}
	if command.Body == nil {
		return view.UploadSession{}, xerrors.New(xerrors.CodeInvalidArgument, "request body is required", xerrors.Details{
			"field": "body",
		})
	}

	session, err := h.sessions.GetByID(ctx, command.Auth.TenantID, command.UploadSessionID)
	if err != nil {
		return view.UploadSession{}, err
	}
	if err := command.Auth.EnsureSessionAccess(session); err != nil {
		return view.UploadSession{}, err
	}
	if session.Mode != domain.SessionModeInline {
		return view.UploadSession{}, newUploadModeMismatch(session.Mode, domain.SessionModeInline)
	}

	contentType := strings.TrimSpace(command.ContentType)
	if contentType != "" && !strings.EqualFold(contentType, session.ContentType) {
		return view.UploadSession{}, xerrors.New(xerrors.CodeInvalidArgument, "content type does not match upload session", xerrors.Details{
			"field":              "contentType",
			"declaredContentType": session.ContentType,
			"contentType":         contentType,
		})
	}

	if err := h.writer.PutObject(ctx, session.Object, session.ContentType, command.Body, session.SizeBytes); err != nil {
		return view.UploadSession{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "upload inline content", err, nil)
	}
	if err := session.MarkUploading(1); err != nil {
		return view.UploadSession{}, err
	}
	session.UpdatedAt = h.clock.Now()
	if err := h.sessions.Save(ctx, session); err != nil {
		return view.UploadSession{}, err
	}

	return view.FromSession(session, "", nil), nil
}
