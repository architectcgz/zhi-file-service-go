package queries

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type ResolveDownloadQuery struct {
	FileID      string
	Disposition domain.DownloadDisposition
	Auth        domain.AuthContext
}

type ResolveDownloadResult struct {
	File domain.FileView
	URL  string
}

type ResolveDownloadHandler struct {
	repo             ports.FileReadRepository
	policies         ports.TenantPolicyReader
	locator          ports.ObjectLocator
	presign          ports.PresignManager
	privateTTL       time.Duration
	publicURLEnabled bool
}

func NewResolveDownloadHandler(
	repo ports.FileReadRepository,
	policies ports.TenantPolicyReader,
	locator ports.ObjectLocator,
	presign ports.PresignManager,
	privateTTL time.Duration,
	publicURLEnabled bool,
) ResolveDownloadHandler {
	return ResolveDownloadHandler{
		repo:             repo,
		policies:         policies,
		locator:          locator,
		presign:          presign,
		privateTTL:       privateTTL,
		publicURLEnabled: publicURLEnabled,
	}
}

func (h ResolveDownloadHandler) Handle(ctx context.Context, query ResolveDownloadQuery) (ResolveDownloadResult, error) {
	if err := query.Auth.RequireFileRead(); err != nil {
		return ResolveDownloadResult{}, err
	}

	file, err := h.repo.GetByID(ctx, query.FileID)
	if err != nil {
		return ResolveDownloadResult{}, err
	}
	if err := file.EnsureReadable(query.Auth); err != nil {
		return ResolveDownloadResult{}, err
	}
	disposition, err := domain.NormalizeDisposition(query.Disposition)
	if err != nil {
		return ResolveDownloadResult{}, err
	}
	policy, err := h.policies.GetByTenantID(ctx, file.TenantID)
	if err != nil {
		return ResolveDownloadResult{}, err
	}
	if err := policy.EnsureAllowed(disposition); err != nil {
		return ResolveDownloadResult{}, err
	}

	result := ResolveDownloadResult{File: file}
	if h.publicURLEnabled && file.AccessLevel == storage.AccessLevelPublic {
		url, err := h.locator.ResolveObjectURL(file.ObjectRef())
		if err != nil {
			return ResolveDownloadResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "resolve public object url", err, nil)
		}
		result.URL = url
		return result, nil
	}

	url, err := h.presign.PresignGetObject(ctx, file.ObjectRef(), h.privateTTL)
	if err != nil {
		return ResolveDownloadResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "presign private object", err, nil)
	}
	result.URL = url

	return result, nil
}
