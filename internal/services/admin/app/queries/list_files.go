package queries

import (
	"context"

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
	ListDefaultLimit int
	ListMaxLimit     int
}

type ListFilesHandler struct {
	files        ports.AdminFileRepository
	defaultLimit int
	maxLimit     int
}

func NewListFilesHandler(files ports.AdminFileRepository, cfg ListFilesConfig) ListFilesHandler {
	defaultLimit, maxLimit := normalizeListConfig(cfg.ListDefaultLimit, cfg.ListMaxLimit)

	return ListFilesHandler{
		files:        files,
		defaultLimit: defaultLimit,
		maxLimit:     maxLimit,
	}
}

func (h ListFilesHandler) Handle(ctx context.Context, query ListFilesQuery) (view.AdminFileList, error) {
	if err := authorizeOperation(query.Auth, domain.OperationListFiles); err != nil {
		return view.AdminFileList{}, err
	}

	tenantID, err := normalizeOptionalQueryValue(query.TenantID, "tenantId")
	if err != nil {
		return view.AdminFileList{}, err
	}
	status, err := normalizeFileStatus(query.Status)
	if err != nil {
		return view.AdminFileList{}, err
	}
	cursor, err := normalizeOptionalQueryValue(query.Cursor, "cursor")
	if err != nil {
		return view.AdminFileList{}, err
	}
	if tenantID != "" {
		if err := domain.EnsureTenantScope(query.Auth, tenantID); err != nil {
			return view.AdminFileList{}, err
		}
	}

	items, nextCursor, err := h.files.List(ctx, ports.ListFilesQuery{
		TenantID:     tenantID,
		TenantScopes: scopedTenants(query.Auth),
		Status:       status,
		Cursor:       cursor,
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
