package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const fallbackRequestID = "generated-request-id"

type AuthFunc func(*http.Request) (domain.AuthContext, error)

type CreateUploadSessionUseCase interface {
	Handle(context.Context, commands.CreateUploadSessionCommand) (view.UploadSession, error)
}

type GetUploadSessionUseCase interface {
	Handle(context.Context, queries.GetUploadSessionQuery) (view.UploadSession, error)
}

type UploadInlineContentUseCase interface {
	Handle(context.Context, commands.UploadInlineContentCommand) (view.UploadSession, error)
}

type PresignMultipartPartsUseCase interface {
	Handle(context.Context, commands.PresignMultipartPartsCommand) (commands.PresignMultipartPartsResult, error)
}

type ListUploadedPartsUseCase interface {
	Handle(context.Context, queries.ListUploadedPartsQuery) (queries.ListUploadedPartsResult, error)
}

type CompleteUploadSessionUseCase interface {
	Handle(context.Context, commands.CompleteUploadSessionCommand) (view.CompletedUploadSession, error)
}

type AbortUploadSessionUseCase interface {
	Handle(context.Context, commands.AbortUploadSessionCommand) (view.UploadSession, error)
}

type Options struct {
	Auth                  AuthFunc
	Metrics               MetricsRecorder
	MaxInlineBodyBytes    int64
	CreateUploadSession   CreateUploadSessionUseCase
	GetUploadSession      GetUploadSessionUseCase
	UploadInlineContent   UploadInlineContentUseCase
	PresignMultipartParts PresignMultipartPartsUseCase
	ListUploadedParts     ListUploadedPartsUseCase
	CompleteUploadSession CompleteUploadSessionUseCase
	AbortUploadSession    AbortUploadSessionUseCase
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
	h.mux.HandleFunc("POST /api/v1/upload-sessions", h.handleCreateUploadSession)
	h.mux.HandleFunc("GET /api/v1/upload-sessions/{uploadSessionId}", h.handleGetUploadSession)
	h.mux.HandleFunc("PUT /api/v1/upload-sessions/{uploadSessionId}/content", h.handleUploadInlineContent)
	h.mux.HandleFunc("POST /api/v1/upload-sessions/{uploadSessionId}/parts/presign", h.handlePresignMultipartParts)
	h.mux.HandleFunc("GET /api/v1/upload-sessions/{uploadSessionId}/parts", h.handleListUploadedParts)
	h.mux.HandleFunc("POST /api/v1/upload-sessions/{uploadSessionId}/complete", h.handleCompleteUploadSession)
	h.mux.HandleFunc("POST /api/v1/upload-sessions/{uploadSessionId}/abort", h.handleAbortUploadSession)
}

func (h *Handler) handleCreateUploadSession(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.CreateUploadSession == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "create upload session handler is not configured", nil))
		return
	}

	var request createUploadSessionRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, requestID, err)
		return
	}

	result, err := h.options.CreateUploadSession.Handle(r.Context(), commands.CreateUploadSessionCommand{
		FileName:       strings.TrimSpace(request.FileName),
		ContentType:    strings.TrimSpace(request.ContentType),
		SizeBytes:      request.SizeBytes,
		ContentHash:    toContentHash(request.ContentHash),
		AccessLevel:    pkgstorage.AccessLevel(strings.TrimSpace(request.AccessLevel)),
		UploadMode:     domain.SessionMode(strings.TrimSpace(request.UploadMode)),
		TotalParts:     request.TotalParts,
		Metadata:       cloneStringMap(request.Metadata),
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		Auth:           auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	h.options.Metrics.RecordSessionCreate()

	writeJSON(w, http.StatusCreated, uploadSessionEnvelopeResponse{
		RequestID: requestID,
		Data:      mapUploadSession(result),
	})
}

func (h *Handler) handleGetUploadSession(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.GetUploadSession == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "get upload session handler is not configured", nil))
		return
	}

	result, err := h.options.GetUploadSession.Handle(r.Context(), queries.GetUploadSessionQuery{
		UploadSessionID: strings.TrimSpace(r.PathValue("uploadSessionId")),
		Auth:            auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, uploadSessionEnvelopeResponse{
		RequestID: requestID,
		Data:      mapUploadSession(result),
	})
}

func (h *Handler) handleUploadInlineContent(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.UploadInlineContent == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "upload inline content handler is not configured", nil))
		return
	}
	if h.options.MaxInlineBodyBytes > 0 && r.ContentLength > h.options.MaxInlineBodyBytes {
		writeError(w, requestID, xerrors.New(xerrors.CodePayloadTooLarge, "inline upload payload exceeds configured limit", xerrors.Details{
			"limit": h.options.MaxInlineBodyBytes,
			"field": "sizeBytes",
		}))
		return
	}

	body := r.Body
	if body == nil {
		body = http.NoBody
	}

	result, err := h.options.UploadInlineContent.Handle(r.Context(), commands.UploadInlineContentCommand{
		UploadSessionID: strings.TrimSpace(r.PathValue("uploadSessionId")),
		ContentType:     strings.TrimSpace(r.Header.Get("Content-Type")),
		Body:            body,
		Auth:            auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, uploadSessionEnvelopeResponse{
		RequestID: requestID,
		Data:      mapUploadSession(result),
	})
}

func (h *Handler) handlePresignMultipartParts(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.PresignMultipartParts == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "presign multipart parts handler is not configured", nil))
		return
	}

	var request presignMultipartPartsRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, requestID, err)
		return
	}

	parts := make([]commands.PresignMultipartPart, 0, len(request.Parts))
	for _, part := range request.Parts {
		parts = append(parts, commands.PresignMultipartPart{PartNumber: part.PartNumber})
	}

	result, err := h.options.PresignMultipartParts.Handle(r.Context(), commands.PresignMultipartPartsCommand{
		UploadSessionID: strings.TrimSpace(r.PathValue("uploadSessionId")),
		ExpiresIn:       time.Duration(request.ExpiresInSeconds) * time.Second,
		Parts:           parts,
		Auth:            auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}

	responseParts := make([]presignedPartResponse, 0, len(result.Parts))
	for _, part := range result.Parts {
		responseParts = append(responseParts, presignedPartResponse{
			PartNumber: part.PartNumber,
			URL:        part.URL,
			Headers:    cloneStringMap(part.Headers),
			ExpiresAt:  part.ExpiresAt,
		})
	}

	writeJSON(w, http.StatusOK, presignMultipartPartsEnvelopeResponse{
		RequestID: requestID,
		Data: presignMultipartPartsDataResponse{
			Parts: responseParts,
		},
	})
}

func (h *Handler) handleListUploadedParts(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.ListUploadedParts == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "list uploaded parts handler is not configured", nil))
		return
	}

	result, err := h.options.ListUploadedParts.Handle(r.Context(), queries.ListUploadedPartsQuery{
		UploadSessionID: strings.TrimSpace(r.PathValue("uploadSessionId")),
		Auth:            auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}

	responseParts := make([]uploadedPartResponse, 0, len(result.Parts))
	for _, part := range result.Parts {
		responseParts = append(responseParts, uploadedPartResponse{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
			SizeBytes:  part.SizeBytes,
		})
	}

	writeJSON(w, http.StatusOK, uploadedPartsEnvelopeResponse{
		RequestID: requestID,
		Data: uploadedPartsDataResponse{
			Parts: responseParts,
		},
	})
}

func (h *Handler) handleCompleteUploadSession(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.CompleteUploadSession == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "complete upload session handler is not configured", nil))
		return
	}

	var request completeUploadSessionRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, requestID, err)
		return
	}

	uploadedParts := make([]pkgstorage.UploadedPart, 0, len(request.UploadedParts))
	for _, part := range request.UploadedParts {
		uploadedParts = append(uploadedParts, pkgstorage.UploadedPart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
			SizeBytes:  part.SizeBytes,
		})
	}

	startedAt := time.Now()
	result, err := h.options.CompleteUploadSession.Handle(r.Context(), commands.CompleteUploadSessionCommand{
		UploadSessionID: strings.TrimSpace(r.PathValue("uploadSessionId")),
		UploadedParts:   uploadedParts,
		ContentHash:     toContentHash(request.ContentHash),
		IdempotencyKey:  strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		RequestID:       requestID,
		Auth:            auth,
	})
	if err != nil {
		h.options.Metrics.RecordSessionCompleteFailure(xerrors.CodeOf(err))
		writeError(w, requestID, err)
		return
	}
	h.options.Metrics.RecordSessionComplete(time.Since(startedAt))

	writeJSON(w, http.StatusOK, completedUploadSessionEnvelopeResponse{
		RequestID: requestID,
		Data: completedUploadSessionDataResponse{
			UploadSession: mapUploadSession(result.UploadSession),
			FileID:        result.FileID,
		},
	})
}

func (h *Handler) handleAbortUploadSession(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.AbortUploadSession == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "abort upload session handler is not configured", nil))
		return
	}

	var request abortUploadSessionRequest
	if hasBody(r) {
		if err := decodeJSON(r.Body, &request); err != nil {
			writeError(w, requestID, err)
			return
		}
	}

	result, err := h.options.AbortUploadSession.Handle(r.Context(), commands.AbortUploadSessionCommand{
		UploadSessionID: strings.TrimSpace(r.PathValue("uploadSessionId")),
		Reason:          strings.TrimSpace(request.Reason),
		IdempotencyKey:  strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		Auth:            auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	h.options.Metrics.RecordSessionAbort()

	writeJSON(w, http.StatusOK, uploadSessionEnvelopeResponse{
		RequestID: requestID,
		Data:      mapUploadSession(result),
	})
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

func writeError(w http.ResponseWriter, requestID string, err error) {
	var coded *xerrors.Error
	if !errors.As(err, &coded) {
		err = xerrors.Wrap(xerrors.CodeInternalError, "internal server error", err, nil)
		_ = errors.As(err, &coded)
	}
	if requestID == "" {
		requestID = fallbackRequestID
	}

	payload := errorEnvelopeResponse{
		RequestID: requestID,
		Error: errorPayload{
			Code:    string(coded.Code),
			Message: coded.Message,
			Details: map[string]any(coded.Details),
		},
	}
	writeJSON(w, coded.HTTPStatus(), payload)
}

func responseRequestID(payload any) string {
	switch value := payload.(type) {
	case uploadSessionEnvelopeResponse:
		return value.RequestID
	case completedUploadSessionEnvelopeResponse:
		return value.RequestID
	case presignMultipartPartsEnvelopeResponse:
		return value.RequestID
	case uploadedPartsEnvelopeResponse:
		return value.RequestID
	case errorEnvelopeResponse:
		return value.RequestID
	default:
		return ""
	}
}

func toContentHash(hash *contentHashRequest) *domain.ContentHash {
	if hash == nil {
		return nil
	}
	return &domain.ContentHash{
		Algorithm: strings.TrimSpace(hash.Algorithm),
		Value:     strings.TrimSpace(hash.Value),
	}
}

func cloneStringMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(value))
	for key, entry := range value {
		cloned[key] = entry
	}
	return cloned
}

func mapUploadSession(session view.UploadSession) uploadSessionResponse {
	return uploadSessionResponse{
		UploadSessionID: session.UploadSessionID,
		TenantID:        session.TenantID,
		UploadMode:      string(session.UploadMode),
		Status:          string(session.Status),
		FileName:        session.FileName,
		ContentType:     session.ContentType,
		SizeBytes:       session.SizeBytes,
		AccessLevel:     string(session.AccessLevel),
		TotalParts:      session.TotalParts,
		UploadedParts:   session.UploadedParts,
		PutURL:          session.PutURL,
		PutHeaders:      cloneStringMap(session.PutHeaders),
		FileID:          session.FileID,
		CreatedAt:       session.CreatedAt,
		UpdatedAt:       session.UpdatedAt,
		CompletedAt:     session.CompletedAt,
	}
}

type createUploadSessionRequest struct {
	FileName    string              `json:"fileName"`
	ContentType string              `json:"contentType"`
	SizeBytes   int64               `json:"sizeBytes"`
	ContentHash *contentHashRequest `json:"contentHash"`
	AccessLevel string              `json:"accessLevel"`
	UploadMode  string              `json:"uploadMode"`
	TotalParts  int                 `json:"totalParts"`
	Metadata    map[string]string   `json:"metadata"`
}

type contentHashRequest struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type presignMultipartPartsRequest struct {
	ExpiresInSeconds int                  `json:"expiresInSeconds"`
	Parts            []presignPartRequest `json:"parts"`
}

type presignPartRequest struct {
	PartNumber int `json:"partNumber"`
}

type completeUploadSessionRequest struct {
	UploadedParts []uploadedPartRequest `json:"uploadedParts"`
	ContentHash   *contentHashRequest   `json:"contentHash"`
}

type uploadedPartRequest struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
	SizeBytes  int64  `json:"sizeBytes"`
}

type abortUploadSessionRequest struct {
	Reason string `json:"reason"`
}

type uploadSessionEnvelopeResponse struct {
	RequestID string                `json:"requestId"`
	Data      uploadSessionResponse `json:"data"`
}

type completedUploadSessionEnvelopeResponse struct {
	RequestID string                             `json:"requestId"`
	Data      completedUploadSessionDataResponse `json:"data"`
}

type completedUploadSessionDataResponse struct {
	UploadSession uploadSessionResponse `json:"uploadSession"`
	FileID        string                `json:"fileId"`
}

type presignMultipartPartsEnvelopeResponse struct {
	RequestID string                            `json:"requestId"`
	Data      presignMultipartPartsDataResponse `json:"data"`
}

type presignMultipartPartsDataResponse struct {
	Parts []presignedPartResponse `json:"parts"`
}

type uploadedPartsEnvelopeResponse struct {
	RequestID string                    `json:"requestId"`
	Data      uploadedPartsDataResponse `json:"data"`
}

type uploadedPartsDataResponse struct {
	Parts []uploadedPartResponse `json:"parts"`
}

type uploadSessionResponse struct {
	UploadSessionID string            `json:"uploadSessionId"`
	TenantID        string            `json:"tenantId"`
	UploadMode      string            `json:"uploadMode"`
	Status          string            `json:"status"`
	FileName        string            `json:"fileName"`
	ContentType     string            `json:"contentType"`
	SizeBytes       int64             `json:"sizeBytes,omitempty"`
	AccessLevel     string            `json:"accessLevel"`
	TotalParts      int               `json:"totalParts,omitempty"`
	UploadedParts   int               `json:"uploadedParts,omitempty"`
	PutURL          string            `json:"putUrl,omitempty"`
	PutHeaders      map[string]string `json:"putHeaders,omitempty"`
	FileID          string            `json:"fileId,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
	CompletedAt     *time.Time        `json:"completedAt,omitempty"`
}

type presignedPartResponse struct {
	PartNumber int               `json:"partNumber"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers,omitempty"`
	ExpiresAt  time.Time         `json:"expiresAt"`
}

type uploadedPartResponse struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
	SizeBytes  int64  `json:"sizeBytes,omitempty"`
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
