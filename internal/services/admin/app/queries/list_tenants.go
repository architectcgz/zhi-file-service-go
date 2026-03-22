package queries

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
)

type ListTenantsQuery struct {
	Cursor string
	Limit  int
	Status *domain.TenantStatus
	Auth   domain.AdminContext
}

type ListTenantsConfig struct {
	ListDefaultLimit int
	ListMaxLimit     int
}

type ListTenantsHandler struct {
	tenants      ports.TenantRepository
	defaultLimit int
	maxLimit     int
}

func NewListTenantsHandler(tenants ports.TenantRepository, cfg ListTenantsConfig) ListTenantsHandler {
	defaultLimit, maxLimit := normalizeListConfig(cfg.ListDefaultLimit, cfg.ListMaxLimit)

	return ListTenantsHandler{
		tenants:      tenants,
		defaultLimit: defaultLimit,
		maxLimit:     maxLimit,
	}
}

func (h ListTenantsHandler) Handle(ctx context.Context, query ListTenantsQuery) (view.TenantList, error) {
	if err := authorizeOperation(query.Auth, domain.OperationListTenants); err != nil {
		return view.TenantList{}, err
	}
	if query.Status != nil {
		if err := query.Status.Validate(); err != nil {
			return view.TenantList{}, err
		}
	}

	items, nextCursor, err := h.tenants.List(ctx, ports.ListTenantsQuery{
		Cursor:       query.Cursor,
		Limit:        normalizeLimit(query.Limit, h.defaultLimit, h.maxLimit),
		Status:       query.Status,
		TenantScopes: scopedTenants(query.Auth),
	})
	if err != nil {
		return view.TenantList{}, err
	}

	return view.TenantList{
		Items:      view.FromTenants(items),
		NextCursor: nextCursor,
	}, nil
}

func normalizeLimit(value int, defaultLimit int, maxLimit int) int {
	if value <= 0 {
		value = defaultLimit
	}
	if value > maxLimit {
		value = maxLimit
	}

	return value
}
