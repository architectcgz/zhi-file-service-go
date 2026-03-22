package queries

import (
	"context"
	"fmt"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type GetFileQuery struct {
	FileID string
	Auth   domain.AuthContext
}

type GetFileResult struct {
	File        domain.FileView
	DownloadURL string
}

type GetFileHandler struct {
	repo             ports.FileReadRepository
	locator          ports.ObjectLocator
	publicURLEnabled bool
}

func NewGetFileHandler(repo ports.FileReadRepository, locator ports.ObjectLocator, publicURLEnabled bool) GetFileHandler {
	return GetFileHandler{
		repo:             repo,
		locator:          locator,
		publicURLEnabled: publicURLEnabled,
	}
}

func (h GetFileHandler) Handle(ctx context.Context, query GetFileQuery) (GetFileResult, error) {
	if err := query.Auth.RequireFileRead(); err != nil {
		return GetFileResult{}, err
	}

	file, err := h.repo.GetByID(ctx, query.FileID)
	if err != nil {
		return GetFileResult{}, err
	}
	if err := file.EnsureReadable(query.Auth); err != nil {
		return GetFileResult{}, err
	}

	result := GetFileResult{File: file}
	if h.publicURLEnabled && file.AccessLevel == storage.AccessLevelPublic {
		url, err := h.locator.ResolveObjectURL(file.ObjectRef())
		if err != nil {
			return GetFileResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "resolve public object url", err, nil)
		}
		result.DownloadURL = url
	}

	return result, nil
}

func (r GetFileResult) String() string {
	return fmt.Sprintf("file=%s download_url=%s", r.File.FileID, r.DownloadURL)
}
