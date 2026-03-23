package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const fallbackRequestID = "generated-request-id"

type AuthFunc func(*http.Request) (domain.AuthContext, error)

type GetFileUseCase interface {
	Handle(context.Context, queries.GetFileQuery) (queries.GetFileResult, error)
}

type CreateAccessTicketUseCase interface {
	Handle(context.Context, commands.CreateAccessTicketCommand) (commands.CreateAccessTicketResult, error)
}

type ResolveDownloadUseCase interface {
	Handle(context.Context, queries.ResolveDownloadQuery) (queries.ResolveDownloadResult, error)
}

type RedirectByAccessTicketUseCase interface {
	Handle(context.Context, queries.RedirectByAccessTicketQuery) (queries.RedirectByAccessTicketResult, error)
}

type Options struct {
	Auth                   AuthFunc
	Metrics                MetricsRecorder
	GetFile                GetFileUseCase
	CreateAccessTicket     CreateAccessTicketUseCase
	ResolveDownload        ResolveDownloadUseCase
	RedirectByAccessTicket RedirectByAccessTicketUseCase
}

type Handler struct {
	options Options
	mux     *http.ServeMux
}

func NewHandler(options Options) http.Handler {
	if options.Metrics == nil {
		options.Metrics = noopMetricsRecorder{}
	}
	handler := &Handler{
		options: options,
		mux:     http.NewServeMux(),
	}
	handler.routes()
	return handler.mux
}

func (h *Handler) routes() {
	h.mux.HandleFunc("GET /api/v1/files/{fileId}", h.handleGetFile)
	h.mux.HandleFunc("POST /api/v1/files/{fileId}/access-tickets", h.handleCreateAccessTicket)
	h.mux.HandleFunc("GET /api/v1/files/{fileId}/download", h.handleResolveDownload)
	h.mux.HandleFunc("GET /api/v1/access-tickets/{ticket}/redirect", h.handleRedirectByAccessTicket)
}

func (h *Handler) handleGetFile(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.GetFile == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "get file handler is not configured", nil))
		return
	}

	result, err := h.options.GetFile.Handle(r.Context(), queries.GetFileQuery{
		FileID: strings.TrimSpace(r.PathValue("fileId")),
		Auth:   auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	h.options.Metrics.RecordFileGet()

	writeJSON(w, http.StatusOK, fileEnvelopeResponse{
		RequestID: requestID,
		Data:      mapFileResponse(result.File, result.DownloadURL),
	})
}

func (h *Handler) handleCreateAccessTicket(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.CreateAccessTicket == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "create access ticket handler is not configured", nil))
		return
	}

	var request createAccessTicketRequest
	if hasBody(r) {
		if err := decodeJSON(r.Body, &request); err != nil {
			writeError(w, requestID, err)
			return
		}
	}

	result, err := h.options.CreateAccessTicket.Handle(r.Context(), commands.CreateAccessTicketCommand{
		FileID:         strings.TrimSpace(r.PathValue("fileId")),
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		ExpiresIn:      time.Duration(request.ExpiresInSeconds) * time.Second,
		Disposition:    domain.DownloadDisposition(strings.TrimSpace(request.ResponseDisposition)),
		ResponseName:   strings.TrimSpace(request.ResponseFileName),
		Auth:           auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	h.options.Metrics.RecordAccessTicketIssue()

	writeJSON(w, http.StatusCreated, accessTicketEnvelopeResponse{
		RequestID: requestID,
		Data: accessTicketDataResponse{
			Ticket:      result.Ticket,
			RedirectURL: result.RedirectURL,
			ExpiresAt:   result.ExpiresAt,
		},
	})
}

func (h *Handler) handleResolveDownload(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		h.options.Metrics.RecordDownloadRedirectFailure(xerrors.CodeOf(err))
		writeError(w, requestID, err)
		return
	}
	if h.options.ResolveDownload == nil {
		err = xerrors.New(xerrors.CodeInternalError, "resolve download handler is not configured", nil)
		h.options.Metrics.RecordDownloadRedirectFailure(xerrors.CodeOf(err))
		writeError(w, requestID, err)
		return
	}

	result, err := h.options.ResolveDownload.Handle(r.Context(), queries.ResolveDownloadQuery{
		FileID:      strings.TrimSpace(r.PathValue("fileId")),
		Disposition: domain.DownloadDisposition(strings.TrimSpace(r.URL.Query().Get("disposition"))),
		Auth:        auth,
	})
	if err != nil {
		h.options.Metrics.RecordDownloadRedirectFailure(xerrors.CodeOf(err))
		writeError(w, requestID, err)
		return
	}
	h.options.Metrics.RecordDownloadRedirect()

	writeRedirect(w, requestID, result.URL)
}

func (h *Handler) handleRedirectByAccessTicket(w http.ResponseWriter, r *http.Request) {
	requestID := requestIDFromRequest(r, "")
	if h.options.RedirectByAccessTicket == nil {
		err := xerrors.New(xerrors.CodeInternalError, "redirect by access ticket handler is not configured", nil)
		h.options.Metrics.RecordDownloadRedirectFailure(xerrors.CodeOf(err))
		writeError(w, requestID, err)
		return
	}

	result, err := h.options.RedirectByAccessTicket.Handle(r.Context(), queries.RedirectByAccessTicketQuery{
		Ticket: strings.TrimSpace(r.PathValue("ticket")),
	})
	if err != nil {
		code := xerrors.CodeOf(err)
		h.options.Metrics.RecordDownloadRedirectFailure(code)
		if code == domain.CodeAccessTicketInvalid || code == domain.CodeAccessTicketExpired {
			h.options.Metrics.RecordAccessTicketVerifyFailure(code)
		}
		writeError(w, requestID, err)
		return
	}
	h.options.Metrics.RecordDownloadRedirect()

	writeRedirect(w, requestID, result.URL)
}

func (h *Handler) authenticate(r *http.Request) (domain.AuthContext, string, error) {
	requestID := requestIDFromRequest(r, "")
	token := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(token), "bearer ") || strings.TrimSpace(token[7:]) == "" {
		return domain.AuthContext{}, requestID, xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
	}
	if h.options.Auth == nil {
		return domain.AuthContext{}, requestID, xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
	}

	auth, err := h.options.Auth(r)
	if err != nil {
		return domain.AuthContext{}, requestID, err
	}
	auth.RequestID = requestIDFromRequest(r, auth.RequestID)
	return auth, auth.RequestID, nil
}

func requestIDFromRequest(r *http.Request, fallback string) string {
	if value := strings.TrimSpace(r.Header.Get("X-Request-Id")); value != "" {
		return value
	}
	if value := strings.TrimSpace(fallback); value != "" {
		return value
	}
	return fallbackRequestID
}

func decodeJSON(body io.ReadCloser, target any) error {
	if body == nil {
		return xerrors.New(xerrors.CodeInvalidArgument, "request body is required", xerrors.Details{
			"field": "body",
		})
	}
	defer body.Close()

	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return xerrors.New(xerrors.CodeInvalidArgument, "request validation failed", xerrors.Details{
			"field":  "body",
			"reason": err.Error(),
		})
	}
	return nil
}

func hasBody(r *http.Request) bool {
	return r != nil && r.Body != nil && (r.ContentLength > 0 || r.ContentLength == -1)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if requestID := responseRequestID(payload); requestID != "" {
		w.Header().Set("X-Request-Id", requestID)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeRedirect(w http.ResponseWriter, requestID string, location string) {
	if requestID == "" {
		requestID = fallbackRequestID
	}
	w.Header().Set("X-Request-Id", requestID)
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusFound)
}

func writeError(w http.ResponseWriter, requestID string, err error) {
	var coded *xerrors.Error
	if !errors.As(err, &coded) {
		err = xerrors.Wrap(xerrors.CodeInternalError, "internal server error", err, nil)
		_ = errors.As(err, &coded)
	}
	if requestID == "" {
		requestID = fallbackRequestID
	}

	writeJSON(w, coded.HTTPStatus(), errorEnvelopeResponse{
		RequestID: requestID,
		Error: errorPayload{
			Code:    string(coded.Code),
			Message: coded.Message,
			Details: map[string]any(coded.Details),
		},
	})
}

func responseRequestID(payload any) string {
	switch value := payload.(type) {
	case fileEnvelopeResponse:
		return value.RequestID
	case accessTicketEnvelopeResponse:
		return value.RequestID
	case errorEnvelopeResponse:
		return value.RequestID
	default:
		return ""
	}
}

func mapFileResponse(file domain.FileView, downloadURL string) fileResponse {
	return fileResponse{
		FileID:      file.FileID,
		TenantID:    file.TenantID,
		FileName:    file.FileName,
		ContentType: file.ContentType,
		SizeBytes:   file.SizeBytes,
		AccessLevel: string(file.AccessLevel),
		Status:      string(file.Status),
		DownloadURL: downloadURL,
		CreatedAt:   file.CreatedAt,
		UpdatedAt:   file.UpdatedAt,
	}
}

type createAccessTicketRequest struct {
	ExpiresInSeconds    int    `json:"expiresInSeconds"`
	ResponseDisposition string `json:"responseDisposition"`
	ResponseFileName    string `json:"responseFileName"`
}

type fileEnvelopeResponse struct {
	RequestID string       `json:"requestId"`
	Data      fileResponse `json:"data"`
}

type accessTicketEnvelopeResponse struct {
	RequestID string                   `json:"requestId"`
	Data      accessTicketDataResponse `json:"data"`
}

type fileResponse struct {
	FileID      string    `json:"fileId"`
	TenantID    string    `json:"tenantId"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType,omitempty"`
	SizeBytes   int64     `json:"sizeBytes"`
	AccessLevel string    `json:"accessLevel"`
	Status      string    `json:"status"`
	DownloadURL string    `json:"downloadUrl,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type accessTicketDataResponse struct {
	Ticket      string    `json:"ticket"`
	RedirectURL string    `json:"redirectUrl,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type errorEnvelopeResponse struct {
	RequestID string       `json:"requestId"`
	Error     errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}
