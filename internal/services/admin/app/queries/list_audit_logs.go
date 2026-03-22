package queries

import (
	"context"
	"strings"

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
	DefaultLimit int
	MaxLimit     int
}

type ListAuditLogsHandler struct {
	audits       ports.AuditLogRepository
	defaultLimit int
	maxLimit     int
}

func NewListAuditLogsHandler(audits ports.AuditLogRepository, cfg ListAuditLogsConfig) ListAuditLogsHandler {
	if cfg.DefaultLimit <= 0 {
		cfg.DefaultLimit = 50
	}
	if cfg.MaxLimit <= 0 {
		cfg.MaxLimit = 100
	}

	return ListAuditLogsHandler{
		audits:       audits,
		defaultLimit: cfg.DefaultLimit,
		maxLimit:     cfg.MaxLimit,
	}
}

func (h ListAuditLogsHandler) Handle(ctx context.Context, query ListAuditLogsQuery) (view.AuditLogList, error) {
	if err := authorizeOperation(query.Auth, domain.OperationListAuditLogs); err != nil {
		return view.AuditLogList{}, err
	}

	tenantID := strings.TrimSpace(query.TenantID)
	if tenantID != "" {
		if err := domain.EnsureTenantScope(query.Auth, tenantID); err != nil {
			return view.AuditLogList{}, err
		}
	}

	items, nextCursor, err := h.audits.List(ctx, ports.ListAuditLogsQuery{
		TenantID:     tenantID,
		TenantScopes: scopedTenants(query.Auth),
		ActorID:      strings.TrimSpace(query.ActorID),
		Action:       strings.TrimSpace(query.Action),
		Cursor:       query.Cursor,
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
