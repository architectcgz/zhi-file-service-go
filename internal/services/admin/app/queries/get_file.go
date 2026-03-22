package queries

import (
	"context"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type GetFileQuery struct {
	FileID string
	Auth   domain.AdminContext
}

type GetFileHandler struct {
	files ports.AdminFileRepository
}

func NewGetFileHandler(files ports.AdminFileRepository) GetFileHandler {
	return GetFileHandler{files: files}
}

func (h GetFileHandler) Handle(ctx context.Context, query GetFileQuery) (view.AdminFile, error) {
	fileID := strings.TrimSpace(query.FileID)
	if fileID == "" {
		return view.AdminFile{}, xerrors.New(xerrors.CodeInvalidArgument, "file id is required", xerrors.Details{
			"field": "fileId",
		})
	}
	if err := authorizeOperation(query.Auth, domain.OperationGetFile); err != nil {
		return view.AdminFile{}, err
	}

	file, err := h.files.GetByID(ctx, fileID)
	if err != nil {
		return view.AdminFile{}, err
	}
	if file == nil {
		return view.AdminFile{}, domain.ErrFileNotFound(fileID)
	}
	if err := domain.EnsureTenantScope(query.Auth, file.TenantID); err != nil {
		return view.AdminFile{}, err
	}

	return view.FromAdminFile(file), nil
}
