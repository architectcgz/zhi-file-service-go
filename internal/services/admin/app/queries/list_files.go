package queries

import (
	"context"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
)

type ListFilesQuery struct {
	TenantID string
	Status   string
	Cursor   string
	Limit    int
	Auth     domain.AdminContext
}

type ListFilesConfig struct {
	DefaultLimit int
	MaxLimit     int
}

type ListFilesHandler struct {
	files        ports.AdminFileRepository
	defaultLimit int
	maxLimit     int
}

func NewListFilesHandler(files ports.AdminFileRepository, cfg ListFilesConfig) ListFilesHandler {
	if cfg.DefaultLimit <= 0 {
		cfg.DefaultLimit = 50
	}
	if cfg.MaxLimit <= 0 {
		cfg.MaxLimit = 100
	}

	return ListFilesHandler{
		files:        files,
		defaultLimit: cfg.DefaultLimit,
		maxLimit:     cfg.MaxLimit,
	}
}

func (h ListFilesHandler) Handle(ctx context.Context, query ListFilesQuery) (view.AdminFileList, error) {
	if err := authorizeOperation(query.Auth, domain.OperationListFiles); err != nil {
		return view.AdminFileList{}, err
	}

	tenantID := strings.TrimSpace(query.TenantID)
	if tenantID != "" {
		if err := domain.EnsureTenantScope(query.Auth, tenantID); err != nil {
			return view.AdminFileList{}, err
		}
	}

	items, nextCursor, err := h.files.List(ctx, ports.ListFilesQuery{
		TenantID:     tenantID,
		TenantScopes: scopedTenants(query.Auth),
		Status:       strings.TrimSpace(query.Status),
		Cursor:       query.Cursor,
		Limit:        normalizeLimit(query.Limit, h.defaultLimit, h.maxLimit),
	})
	if err != nil {
		return view.AdminFileList{}, err
	}

	return view.AdminFileList{
		Items:      view.FromAdminFiles(items),
		NextCursor: nextCursor,
	}, nil
}
