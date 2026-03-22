package queries

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
)

type ListAuditLogsQuery struct {
	TenantID string
	ActorID  string
	Action   string
	Cursor   string
	Limit    int
	Auth     domain.AdminContext
}

type ListAuditLogsConfig struct {
	ListDefaultLimit int
	ListMaxLimit     int
}

type ListAuditLogsHandler struct {
	audits       ports.AuditLogRepository
	defaultLimit int
	maxLimit     int
}

func NewListAuditLogsHandler(audits ports.AuditLogRepository, cfg ListAuditLogsConfig) ListAuditLogsHandler {
	defaultLimit, maxLimit := normalizeListConfig(cfg.ListDefaultLimit, cfg.ListMaxLimit)

	return ListAuditLogsHandler{
		audits:       audits,
		defaultLimit: defaultLimit,
		maxLimit:     maxLimit,
	}
}

func (h ListAuditLogsHandler) Handle(ctx context.Context, query ListAuditLogsQuery) (view.AuditLogList, error) {
	if err := authorizeOperation(query.Auth, domain.OperationListAuditLogs); err != nil {
		return view.AuditLogList{}, err
	}

	tenantID, err := normalizeOptionalAuditQueryValue(query.TenantID, "tenantId")
	if err != nil {
		return view.AuditLogList{}, err
	}
	actorID, err := normalizeOptionalAuditQueryValue(query.ActorID, "actorId")
	if err != nil {
		return view.AuditLogList{}, err
	}
	action, err := normalizeOptionalAuditQueryValue(query.Action, "action")
	if err != nil {
		return view.AuditLogList{}, err
	}
	cursor, err := normalizeOptionalAuditQueryValue(query.Cursor, "cursor")
	if err != nil {
		return view.AuditLogList{}, err
	}
	if tenantID != "" {
		if err := domain.EnsureTenantScope(query.Auth, tenantID); err != nil {
			return view.AuditLogList{}, err
		}
	}

	items, nextCursor, err := h.audits.List(ctx, ports.ListAuditLogsQuery{
		TenantID:     tenantID,
		TenantScopes: scopedTenants(query.Auth),
		ActorID:      actorID,
		Action:       action,
		Cursor:       cursor,
		Limit:        normalizeLimit(query.Limit, h.defaultLimit, h.maxLimit),
	})
	if err != nil {
		return view.AuditLogList{}, err
	}

	return view.AuditLogList{
		Items:      view.FromAuditLogs(items),
		NextCursor: nextCursor,
	}, nil
}
