package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const fallbackRequestID = "generated-request-id"

type AuthFunc func(*http.Request) (domain.AdminContext, error)

type CreateTenantUseCase interface {
	Handle(context.Context, commands.CreateTenantCommand) (view.Tenant, error)
}

type PatchTenantUseCase interface {
	Handle(context.Context, commands.PatchTenantCommand) (view.Tenant, error)
}

type PatchTenantPolicyUseCase interface {
	Handle(context.Context, commands.PatchTenantPolicyCommand) (view.TenantPolicy, error)
}

type DeleteFileUseCase interface {
	Handle(context.Context, commands.DeleteFileCommand) (view.DeleteFileResult, error)
}

type ListTenantsUseCase interface {
	Handle(context.Context, queries.ListTenantsQuery) (view.TenantList, error)
}

type GetTenantUseCase interface {
	Handle(context.Context, queries.GetTenantQuery) (view.Tenant, error)
}

type GetTenantPolicyUseCase interface {
	Handle(context.Context, queries.GetTenantPolicyQuery) (view.TenantPolicy, error)
}

type GetTenantUsageUseCase interface {
	Handle(context.Context, queries.GetTenantUsageQuery) (view.TenantUsage, error)
}

type ListFilesUseCase interface {
	Handle(context.Context, queries.ListFilesQuery) (view.AdminFileList, error)
}

type GetFileUseCase interface {
	Handle(context.Context, queries.GetFileQuery) (view.AdminFile, error)
}

type ListAuditLogsUseCase interface {
	Handle(context.Context, queries.ListAuditLogsQuery) (view.AuditLogList, error)
}

type Options struct {
	Auth              AuthFunc
	CreateTenant      CreateTenantUseCase
	ListTenants       ListTenantsUseCase
	GetTenant         GetTenantUseCase
	PatchTenant       PatchTenantUseCase
	GetTenantPolicy   GetTenantPolicyUseCase
	PatchTenantPolicy PatchTenantPolicyUseCase
	GetTenantUsage    GetTenantUsageUseCase
	ListFiles         ListFilesUseCase
	GetFile           GetFileUseCase
	DeleteFile        DeleteFileUseCase
	ListAuditLogs     ListAuditLogsUseCase
}

type Handler struct {
	options Options
	mux     *http.ServeMux
}

func NewHandler(options Options) http.Handler {
	handler := &Handler{
		options: options,
		mux:     http.NewServeMux(),
	}
	handler.routes()
	return handler.mux
}

func (h *Handler) routes() {
	h.mux.HandleFunc("GET /api/admin/v1/tenants", h.handleListTenants)
	h.mux.HandleFunc("POST /api/admin/v1/tenants", h.handleCreateTenant)
	h.mux.HandleFunc("GET /api/admin/v1/tenants/{tenantId}", h.handleGetTenant)
	h.mux.HandleFunc("PATCH /api/admin/v1/tenants/{tenantId}", h.handlePatchTenant)
	h.mux.HandleFunc("GET /api/admin/v1/tenants/{tenantId}/policy", h.handleGetTenantPolicy)
	h.mux.HandleFunc("PATCH /api/admin/v1/tenants/{tenantId}/policy", h.handlePatchTenantPolicy)
	h.mux.HandleFunc("GET /api/admin/v1/tenants/{tenantId}/usage", h.handleGetTenantUsage)
	h.mux.HandleFunc("GET /api/admin/v1/files", h.handleListFiles)
	h.mux.HandleFunc("GET /api/admin/v1/files/{fileId}", h.handleGetFile)
	h.mux.HandleFunc("DELETE /api/admin/v1/files/{fileId}", h.handleDeleteFile)
	h.mux.HandleFunc("GET /api/admin/v1/audit-logs", h.handleListAuditLogs)
}

func (h *Handler) handleListTenants(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.ListTenants == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "list tenants handler is not configured", nil))
		return
	}

	query, err := parseListTenantsQuery(r, auth)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	result, err := h.options.ListTenants.Handle(r.Context(), query)
	if err != nil {
		writeError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, tenantListEnvelopeResponse{
		RequestID: requestID,
		Data:      mapTenants(result.Items),
		Page:      pageInfoResponse{NextCursor: result.NextCursor},
	})
}

func (h *Handler) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.CreateTenant == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "create tenant handler is not configured", nil))
		return
	}

	var request createTenantRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, requestID, err)
		return
	}
	result, err := h.options.CreateTenant.Handle(r.Context(), commands.CreateTenantCommand{
		TenantID:       strings.TrimSpace(request.TenantID),
		TenantName:     strings.TrimSpace(request.TenantName),
		ContactEmail:   strings.TrimSpace(request.ContactEmail),
		Description:    strings.TrimSpace(request.Description),
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		Auth:           auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusCreated, tenantEnvelopeResponse{
		RequestID: requestID,
		Data:      mapTenant(result),
	})
}

func (h *Handler) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.GetTenant == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "get tenant handler is not configured", nil))
		return
	}

	result, err := h.options.GetTenant.Handle(r.Context(), queries.GetTenantQuery{
		TenantID: strings.TrimSpace(r.PathValue("tenantId")),
		Auth:     auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, tenantEnvelopeResponse{
		RequestID: requestID,
		Data:      mapTenant(result),
	})
}

func (h *Handler) handlePatchTenant(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.PatchTenant == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "patch tenant handler is not configured", nil))
		return
	}

	var request patchTenantRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, requestID, err)
		return
	}
	result, err := h.options.PatchTenant.Handle(r.Context(), commands.PatchTenantCommand{
		TenantID: strings.TrimSpace(r.PathValue("tenantId")),
		Patch: domain.TenantPatch{
			TenantName:   trimStringPtr(request.TenantName),
			Status:       toTenantStatusPtr(request.Status),
			ContactEmail: trimStringPtr(request.ContactEmail),
			Description:  trimStringPtr(request.Description),
			Reason:       strings.TrimSpace(request.Reason),
		},
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		Auth:           auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, tenantEnvelopeResponse{
		RequestID: requestID,
		Data:      mapTenant(result),
	})
}

func (h *Handler) handleGetTenantPolicy(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.GetTenantPolicy == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "get tenant policy handler is not configured", nil))
		return
	}

	result, err := h.options.GetTenantPolicy.Handle(r.Context(), queries.GetTenantPolicyQuery{
		TenantID: strings.TrimSpace(r.PathValue("tenantId")),
		Auth:     auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, tenantPolicyEnvelopeResponse{
		RequestID: requestID,
		Data:      mapTenantPolicy(result),
	})
}

func (h *Handler) handlePatchTenantPolicy(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.PatchTenantPolicy == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "patch tenant policy handler is not configured", nil))
		return
	}

	var request patchTenantPolicyRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, requestID, err)
		return
	}
	result, err := h.options.PatchTenantPolicy.Handle(r.Context(), commands.PatchTenantPolicyCommand{
		TenantID: strings.TrimSpace(r.PathValue("tenantId")),
		Patch: domain.TenantPolicyPatch{
			MaxStorageBytes:    request.MaxStorageBytes,
			MaxFileCount:       request.MaxFileCount,
			MaxSingleFileSize:  request.MaxSingleFileSize,
			AllowedMimeTypes:   trimSlice(request.AllowedMimeTypes),
			AllowedExtensions:  trimSlice(request.AllowedExtensions),
			DefaultAccessLevel: trimStringPtr(request.DefaultAccessLevel),
			AutoCreateEnabled:  request.AutoCreateEnabled,
			Reason:             strings.TrimSpace(request.Reason),
		},
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		Auth:           auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, tenantPolicyEnvelopeResponse{
		RequestID: requestID,
		Data:      mapTenantPolicy(result),
	})
}

func (h *Handler) handleGetTenantUsage(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.GetTenantUsage == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "get tenant usage handler is not configured", nil))
		return
	}

	result, err := h.options.GetTenantUsage.Handle(r.Context(), queries.GetTenantUsageQuery{
		TenantID: strings.TrimSpace(r.PathValue("tenantId")),
		Auth:     auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, tenantUsageEnvelopeResponse{
		RequestID: requestID,
		Data:      mapTenantUsage(result),
	})
}

func (h *Handler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.ListFiles == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "list files handler is not configured", nil))
		return
	}

	query, err := parseListFilesQuery(r, auth)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	result, err := h.options.ListFiles.Handle(r.Context(), query)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, adminFileListEnvelopeResponse{
		RequestID: requestID,
		Data:      mapAdminFiles(result.Items),
		Page:      pageInfoResponse{NextCursor: result.NextCursor},
	})
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
	writeJSON(w, http.StatusOK, adminFileEnvelopeResponse{
		RequestID: requestID,
		Data:      mapAdminFile(result),
	})
}

func (h *Handler) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.DeleteFile == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "delete file handler is not configured", nil))
		return
	}

	var request deleteFileRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, requestID, err)
		return
	}
	result, err := h.options.DeleteFile.Handle(r.Context(), commands.DeleteFileCommand{
		FileID:         strings.TrimSpace(r.PathValue("fileId")),
		Reason:         strings.TrimSpace(request.Reason),
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		Auth:           auth,
	})
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, deleteFileEnvelopeResponse{
		RequestID: requestID,
		Data:      mapDeleteFileResult(result),
	})
}

func (h *Handler) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	auth, requestID, err := h.authenticate(r)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	if h.options.ListAuditLogs == nil {
		writeError(w, requestID, xerrors.New(xerrors.CodeInternalError, "list audit logs handler is not configured", nil))
		return
	}

	query, err := parseListAuditLogsQuery(r, auth)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	result, err := h.options.ListAuditLogs.Handle(r.Context(), query)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, auditLogListEnvelopeResponse{
		RequestID: requestID,
		Data:      mapAuditLogs(result.Items),
		Page:      pageInfoResponse{NextCursor: result.NextCursor},
	})
}

func (h *Handler) authenticate(r *http.Request) (domain.AdminContext, string, error) {
	requestID := requestIDFromRequest(r, "")
	token := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(token), "bearer ") || strings.TrimSpace(token[7:]) == "" {
		return domain.AdminContext{}, requestID, xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
	}
	if h.options.Auth == nil {
		return domain.AdminContext{}, requestID, xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
	}

	auth, err := h.options.Auth(r)
	if err != nil {
		return domain.AdminContext{}, requestID, err
	}
	auth.RequestID = requestIDFromRequest(r, auth.RequestID)
	return auth, auth.RequestID, nil
}

func parseListTenantsQuery(r *http.Request, auth domain.AdminContext) (queries.ListTenantsQuery, error) {
	limit, err := intQueryValue(r, "limit")
	if err != nil {
		return queries.ListTenantsQuery{}, err
	}
	var status *domain.TenantStatus
	if value := strings.TrimSpace(r.URL.Query().Get("status")); value != "" {
		parsed := domain.TenantStatus(value)
		status = &parsed
	}
	return queries.ListTenantsQuery{
		Cursor: strings.TrimSpace(r.URL.Query().Get("cursor")),
		Limit:  limit,
		Status: status,
		Auth:   auth,
	}, nil
}

func parseListFilesQuery(r *http.Request, auth domain.AdminContext) (queries.ListFilesQuery, error) {
	limit, err := intQueryValue(r, "limit")
	if err != nil {
		return queries.ListFilesQuery{}, err
	}
	return queries.ListFilesQuery{
		TenantID: strings.TrimSpace(r.URL.Query().Get("tenantId")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Cursor:   strings.TrimSpace(r.URL.Query().Get("cursor")),
		Limit:    limit,
		Auth:     auth,
	}, nil
}

func parseListAuditLogsQuery(r *http.Request, auth domain.AdminContext) (queries.ListAuditLogsQuery, error) {
	limit, err := intQueryValue(r, "limit")
	if err != nil {
		return queries.ListAuditLogsQuery{}, err
	}
	return queries.ListAuditLogsQuery{
		TenantID: strings.TrimSpace(r.URL.Query().Get("tenantId")),
		ActorID:  strings.TrimSpace(r.URL.Query().Get("actorId")),
		Action:   strings.TrimSpace(r.URL.Query().Get("action")),
		Cursor:   strings.TrimSpace(r.URL.Query().Get("cursor")),
		Limit:    limit,
		Auth:     auth,
	}, nil
}

func intQueryValue(r *http.Request, key string) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, xerrors.New(xerrors.CodeInvalidArgument, "query parameter is invalid", xerrors.Details{
			"field":  key,
			"reason": "must_be_integer",
		})
	}
	return parsed, nil
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
	case tenantEnvelopeResponse:
		return value.RequestID
	case tenantPolicyEnvelopeResponse:
		return value.RequestID
	case tenantUsageEnvelopeResponse:
		return value.RequestID
	case tenantListEnvelopeResponse:
		return value.RequestID
	case adminFileEnvelopeResponse:
		return value.RequestID
	case adminFileListEnvelopeResponse:
		return value.RequestID
	case deleteFileEnvelopeResponse:
		return value.RequestID
	case auditLogListEnvelopeResponse:
		return value.RequestID
	case errorEnvelopeResponse:
		return value.RequestID
	default:
		return ""
	}
}

func mapTenant(value view.Tenant) tenantResponse {
	return tenantResponse{
		TenantID:     value.TenantID,
		TenantName:   value.TenantName,
		Status:       string(value.Status),
		ContactEmail: value.ContactEmail,
		Description:  value.Description,
		CreatedAt:    toTime(value.CreatedAt),
		UpdatedAt:    toTime(value.UpdatedAt),
	}
}

func mapTenants(values []view.Tenant) []tenantResponse {
	items := make([]tenantResponse, 0, len(values))
	for _, value := range values {
		items = append(items, mapTenant(value))
	}
	return items
}

func mapTenantPolicy(value view.TenantPolicy) tenantPolicyResponse {
	return tenantPolicyResponse{
		TenantID:           value.TenantID,
		MaxStorageBytes:    value.MaxStorageBytes,
		MaxFileCount:       value.MaxFileCount,
		MaxSingleFileSize:  value.MaxSingleFileSize,
		AllowedMimeTypes:   cloneStrings(value.AllowedMimeTypes),
		AllowedExtensions:  cloneStrings(value.AllowedExtensions),
		DefaultAccessLevel: value.DefaultAccessLevel,
		AutoCreateEnabled:  value.AutoCreateEnabled,
		CreatedAt:          toTime(value.CreatedAt),
		UpdatedAt:          toTime(value.UpdatedAt),
	}
}

func mapTenantUsage(value view.TenantUsage) tenantUsageResponse {
	return tenantUsageResponse{
		TenantID:         value.TenantID,
		UsedStorageBytes: value.UsedStorageBytes,
		UsedFileCount:    value.UsedFileCount,
		LastUploadAt:     value.LastUploadAt,
		UpdatedAt:        toTime(value.UpdatedAt),
	}
}

func mapAdminFile(value view.AdminFile) adminFileResponse {
	return adminFileResponse{
		FileID:      value.FileID,
		TenantID:    value.TenantID,
		FileName:    value.FileName,
		ContentType: value.ContentType,
		SizeBytes:   value.SizeBytes,
		AccessLevel: string(value.AccessLevel),
		Status:      value.Status,
		DeletedAt:   value.DeletedAt,
		CreatedAt:   toTime(value.CreatedAt),
		UpdatedAt:   toTime(value.UpdatedAt),
	}
}

func mapAdminFiles(values []view.AdminFile) []adminFileResponse {
	items := make([]adminFileResponse, 0, len(values))
	for _, value := range values {
		items = append(items, mapAdminFile(value))
	}
	return items
}

func mapDeleteFileResult(value view.DeleteFileResult) deleteFileResultResponse {
	return deleteFileResultResponse{
		FileID:                  value.FileID,
		Status:                  value.Status,
		DeletedAt:               value.DeletedAt,
		PhysicalDeleteScheduled: value.PhysicalDeleteScheduled,
	}
}

func mapAuditLogs(values []view.AuditLog) []auditLogResponse {
	items := make([]auditLogResponse, 0, len(values))
	for _, value := range values {
		items = append(items, auditLogResponse{
			AuditLogID: value.AuditLogID,
			TenantID:   value.TenantID,
			ActorID:    value.ActorID,
			Action:     value.Action,
			TargetType: value.TargetType,
			TargetID:   value.TargetID,
			Details:    cloneMap(value.Details),
			CreatedAt:  toTime(value.CreatedAt),
		})
	}
	return items
}

func trimStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func trimSlice(values []string) []string {
	if values == nil {
		return nil
	}
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strings.TrimSpace(value))
	}
	return items
}

func toTenantStatusPtr(value *string) *domain.TenantStatus {
	if value == nil {
		return nil
	}
	status := domain.TenantStatus(strings.TrimSpace(*value))
	return &status
}

func toTime(value view.Time) time.Time {
	return time.Time(value).UTC()
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(value))
	for key, entry := range value {
		cloned[key] = entry
	}
	return cloned
}

type createTenantRequest struct {
	TenantID     string `json:"tenantId"`
	TenantName   string `json:"tenantName"`
	ContactEmail string `json:"contactEmail"`
	Description  string `json:"description"`
}

type patchTenantRequest struct {
	TenantName   *string `json:"tenantName"`
	Status       *string `json:"status"`
	ContactEmail *string `json:"contactEmail"`
	Description  *string `json:"description"`
	Reason       string  `json:"reason"`
}

type patchTenantPolicyRequest struct {
	MaxStorageBytes    *int64   `json:"maxStorageBytes"`
	MaxFileCount       *int64   `json:"maxFileCount"`
	MaxSingleFileSize  *int64   `json:"maxSingleFileSize"`
	AllowedMimeTypes   []string `json:"allowedMimeTypes"`
	AllowedExtensions  []string `json:"allowedExtensions"`
	DefaultAccessLevel *string  `json:"defaultAccessLevel"`
	AutoCreateEnabled  *bool    `json:"autoCreateEnabled"`
	Reason             string   `json:"reason"`
}

type deleteFileRequest struct {
	Reason string `json:"reason"`
}

type tenantEnvelopeResponse struct {
	RequestID string         `json:"requestId"`
	Data      tenantResponse `json:"data"`
}

type tenantPolicyEnvelopeResponse struct {
	RequestID string               `json:"requestId"`
	Data      tenantPolicyResponse `json:"data"`
}

type tenantUsageEnvelopeResponse struct {
	RequestID string              `json:"requestId"`
	Data      tenantUsageResponse `json:"data"`
}

type tenantListEnvelopeResponse struct {
	RequestID string           `json:"requestId"`
	Data      []tenantResponse `json:"data"`
	Page      pageInfoResponse `json:"page,omitempty"`
}

type adminFileEnvelopeResponse struct {
	RequestID string            `json:"requestId"`
	Data      adminFileResponse `json:"data"`
}

type adminFileListEnvelopeResponse struct {
	RequestID string              `json:"requestId"`
	Data      []adminFileResponse `json:"data"`
	Page      pageInfoResponse    `json:"page,omitempty"`
}

type deleteFileEnvelopeResponse struct {
	RequestID string                   `json:"requestId"`
	Data      deleteFileResultResponse `json:"data"`
}

type auditLogListEnvelopeResponse struct {
	RequestID string             `json:"requestId"`
	Data      []auditLogResponse `json:"data"`
	Page      pageInfoResponse   `json:"page,omitempty"`
}

type pageInfoResponse struct {
	NextCursor string `json:"nextCursor,omitempty"`
}

type tenantResponse struct {
	TenantID     string    `json:"tenantId"`
	TenantName   string    `json:"tenantName"`
	Status       string    `json:"status"`
	ContactEmail string    `json:"contactEmail,omitempty"`
	Description  string    `json:"description,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type tenantPolicyResponse struct {
	TenantID           string    `json:"tenantId"`
	MaxStorageBytes    *int64    `json:"maxStorageBytes,omitempty"`
	MaxFileCount       *int64    `json:"maxFileCount,omitempty"`
	MaxSingleFileSize  *int64    `json:"maxSingleFileSize,omitempty"`
	AllowedMimeTypes   []string  `json:"allowedMimeTypes,omitempty"`
	AllowedExtensions  []string  `json:"allowedExtensions,omitempty"`
	DefaultAccessLevel *string   `json:"defaultAccessLevel,omitempty"`
	AutoCreateEnabled  *bool     `json:"autoCreateEnabled,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type tenantUsageResponse struct {
	TenantID         string     `json:"tenantId"`
	UsedStorageBytes int64      `json:"usedStorageBytes"`
	UsedFileCount    int64      `json:"usedFileCount"`
	LastUploadAt     *time.Time `json:"lastUploadAt,omitempty"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type adminFileResponse struct {
	FileID      string     `json:"fileId"`
	TenantID    string     `json:"tenantId"`
	FileName    string     `json:"fileName"`
	ContentType string     `json:"contentType,omitempty"`
	SizeBytes   int64      `json:"sizeBytes"`
	AccessLevel string     `json:"accessLevel"`
	Status      string     `json:"status"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type deleteFileResultResponse struct {
	FileID                  string     `json:"fileId"`
	Status                  string     `json:"status"`
	DeletedAt               *time.Time `json:"deletedAt,omitempty"`
	PhysicalDeleteScheduled bool       `json:"physicalDeleteScheduled"`
}

type auditLogResponse struct {
	AuditLogID string         `json:"auditLogId"`
	TenantID   string         `json:"tenantId,omitempty"`
	ActorID    string         `json:"actorId"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	TargetID   string         `json:"targetId,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
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
