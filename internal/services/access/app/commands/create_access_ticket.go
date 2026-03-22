package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type CreateAccessTicketCommand struct {
	FileID       string
	ExpiresIn    time.Duration
	Disposition  domain.DownloadDisposition
	ResponseName string
	Auth         domain.AuthContext
}

type CreateAccessTicketResult struct {
	Ticket      string
	RedirectURL string
	ExpiresAt   time.Time
}

type CreateAccessTicketHandler struct {
	repo             ports.FileReadRepository
	issuer           ports.AccessTicketIssuer
	clock            clock.Clock
	defaultTTL       time.Duration
	redirectBasePath string
}

func NewCreateAccessTicketHandler(
	repo ports.FileReadRepository,
	issuer ports.AccessTicketIssuer,
	clk clock.Clock,
	defaultTTL time.Duration,
	redirectBasePath string,
) CreateAccessTicketHandler {
	if clk == nil {
		clk = clock.SystemClock{}
	}

	return CreateAccessTicketHandler{
		repo:             repo,
		issuer:           issuer,
		clock:            clk,
		defaultTTL:       defaultTTL,
		redirectBasePath: redirectBasePath,
	}
}

func (h CreateAccessTicketHandler) Handle(ctx context.Context, command CreateAccessTicketCommand) (CreateAccessTicketResult, error) {
	if err := command.Auth.RequireFileRead(); err != nil {
		return CreateAccessTicketResult{}, err
	}
	if command.ExpiresIn < 0 {
		return CreateAccessTicketResult{}, xerrors.New(xerrors.CodeInvalidArgument, "expiresIn must be >= 0", xerrors.Details{
			"field": "expiresInSeconds",
		})
	}

	file, err := h.repo.GetByID(ctx, command.FileID)
	if err != nil {
		return CreateAccessTicketResult{}, err
	}
	if err := file.EnsureReadable(command.Auth); err != nil {
		return CreateAccessTicketResult{}, err
	}

	disposition, err := domain.NormalizeDisposition(command.Disposition)
	if err != nil {
		return CreateAccessTicketResult{}, err
	}

	ttl := command.ExpiresIn
	if ttl == 0 {
		ttl = h.defaultTTL
	}
	expiresAt := h.clock.Now().Add(ttl)

	claims := domain.AccessTicketClaims{
		FileID:       file.FileID,
		TenantID:     file.TenantID,
		Subject:      command.Auth.SubjectID,
		SubjectType:  command.Auth.SubjectType,
		Disposition:  disposition,
		ResponseName: command.ResponseName,
		ExpiresAt:    expiresAt,
	}
	if err := claims.Validate(); err != nil {
		return CreateAccessTicketResult{}, err
	}

	ticket, err := h.issuer.Issue(ctx, claims)
	if err != nil {
		return CreateAccessTicketResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "issue access ticket", err, nil)
	}

	return CreateAccessTicketResult{
		Ticket:      ticket,
		RedirectURL: buildRedirectURL(h.redirectBasePath, ticket),
		ExpiresAt:   expiresAt,
	}, nil
}

func buildRedirectURL(basePath, ticket string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	if basePath == "" {
		basePath = "/api/v1/access-tickets"
	}

	return fmt.Sprintf("%s/%s/redirect", basePath, ticket)
}
