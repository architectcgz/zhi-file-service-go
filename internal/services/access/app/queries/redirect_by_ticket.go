package queries

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type RedirectByAccessTicketQuery struct {
	Ticket string
}

type RedirectByAccessTicketResult struct {
	File  domain.FileView
	URL   string
	Claim domain.AccessTicketClaims
}

type RedirectByAccessTicketHandler struct {
	repo             ports.FileReadRepository
	policies         ports.TenantPolicyReader
	issuer           ports.AccessTicketIssuer
	locator          ports.ObjectLocator
	presign          ports.PresignManager
	clock            clock.Clock
	privateTTL       time.Duration
	publicURLEnabled bool
}

func NewRedirectByAccessTicketHandler(
	repo ports.FileReadRepository,
	policies ports.TenantPolicyReader,
	issuer ports.AccessTicketIssuer,
	locator ports.ObjectLocator,
	presign ports.PresignManager,
	clk clock.Clock,
	privateTTL time.Duration,
	publicURLEnabled bool,
) RedirectByAccessTicketHandler {
	if clk == nil {
		clk = clock.SystemClock{}
	}

	return RedirectByAccessTicketHandler{
		repo:             repo,
		policies:         policies,
		issuer:           issuer,
		locator:          locator,
		presign:          presign,
		clock:            clk,
		privateTTL:       privateTTL,
		publicURLEnabled: publicURLEnabled,
	}
}

func (h RedirectByAccessTicketHandler) Handle(ctx context.Context, query RedirectByAccessTicketQuery) (RedirectByAccessTicketResult, error) {
	claims, err := h.issuer.Verify(ctx, query.Ticket)
	if err != nil {
		return RedirectByAccessTicketResult{}, domain.ErrAccessTicketInvalid(query.Ticket)
	}
	if err := claims.Validate(); err != nil {
		return RedirectByAccessTicketResult{}, domain.ErrAccessTicketInvalid(query.Ticket)
	}
	if !claims.ExpiresAt.After(h.clock.Now()) {
		return RedirectByAccessTicketResult{}, domain.ErrAccessTicketExpired(query.Ticket)
	}

	file, err := h.repo.GetByID(ctx, claims.FileID)
	if err != nil {
		return RedirectByAccessTicketResult{}, err
	}
	auth := domain.AuthContext{
		SubjectID:   claims.Subject,
		SubjectType: claims.SubjectType,
		TenantID:    claims.TenantID,
		Scopes:      []string{domain.ScopeFileRead},
	}
	if err := file.EnsureReadable(auth); err != nil {
		return RedirectByAccessTicketResult{}, err
	}
	disposition, err := domain.NormalizeDisposition(claims.Disposition)
	if err != nil {
		return RedirectByAccessTicketResult{}, domain.ErrAccessTicketInvalid(query.Ticket)
	}
	policy, err := h.policies.GetByTenantID(ctx, claims.TenantID)
	if err != nil {
		return RedirectByAccessTicketResult{}, err
	}
	if err := policy.EnsureAllowed(disposition); err != nil {
		return RedirectByAccessTicketResult{}, err
	}

	result := RedirectByAccessTicketResult{
		File:  file,
		Claim: claims,
	}
	if h.publicURLEnabled && file.AccessLevel == storage.AccessLevelPublic {
		url, err := h.locator.ResolveObjectURL(file.ObjectRef())
		if err != nil {
			return RedirectByAccessTicketResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "resolve public object url", err, nil)
		}
		result.URL = url
		return result, nil
	}

	url, err := h.presign.PresignGetObject(ctx, file.ObjectRef(), h.privateTTL)
	if err != nil {
		return RedirectByAccessTicketResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "presign private object", err, nil)
	}
	result.URL = url

	return result, nil
}
