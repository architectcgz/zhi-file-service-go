package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type CreateAccessTicketCommand struct {
	FileID         string
	IdempotencyKey string
	ExpiresIn      time.Duration
	Disposition    domain.DownloadDisposition
	ResponseName   string
	Auth           domain.AuthContext
}

type CreateAccessTicketResult struct {
	Ticket      string
	RedirectURL string
	ExpiresAt   time.Time
}

type CreateAccessTicketHandler struct {
	repo             ports.FileReadRepository
	policies         ports.TenantPolicyReader
	issuer           ports.AccessTicketIssuer
	idempotencyStore ports.AccessTicketIdempotencyStore
	clock            clock.Clock
	defaultTTL       time.Duration
	redirectBasePath string
}

func NewCreateAccessTicketHandler(
	repo ports.FileReadRepository,
	policies ports.TenantPolicyReader,
	issuer ports.AccessTicketIssuer,
	idempotencyStore ports.AccessTicketIdempotencyStore,
	clk clock.Clock,
	defaultTTL time.Duration,
	redirectBasePath string,
) CreateAccessTicketHandler {
	if clk == nil {
		clk = clock.SystemClock{}
	}

	return CreateAccessTicketHandler{
		repo:             repo,
		policies:         policies,
		issuer:           issuer,
		idempotencyStore: idempotencyStore,
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
	idempotencyKey := strings.TrimSpace(command.IdempotencyKey)
	if len(idempotencyKey) > 128 {
		return CreateAccessTicketResult{}, xerrors.New(xerrors.CodeInvalidArgument, "idempotency key is too long", xerrors.Details{
			"field": "Idempotency-Key",
		})
	}
	disposition, err := domain.NormalizeDisposition(command.Disposition)
	if err != nil {
		return CreateAccessTicketResult{}, err
	}
	ttl := command.ExpiresIn
	if ttl == 0 {
		ttl = h.defaultTTL
	}
	storageKey := ""
	fingerprint := ""
	if idempotencyKey != "" {
		if h.idempotencyStore == nil {
			return CreateAccessTicketResult{}, xerrors.New(xerrors.CodeServiceUnavailable, "access ticket idempotency is not available", xerrors.Details{
				"field": "Idempotency-Key",
			})
		}

		storageKey = buildIdempotencyStorageKey(command.Auth, command.FileID, idempotencyKey)
		fingerprint, err = buildIdempotencyFingerprint(command.FileID, command.Auth, disposition, command.ResponseName, ttl)
		if err != nil {
			return CreateAccessTicketResult{}, xerrors.Wrap(xerrors.CodeInternalError, "build access ticket idempotency fingerprint", err, nil)
		}

		existing, found, err := h.idempotencyStore.Get(ctx, storageKey)
		if err != nil {
			return CreateAccessTicketResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "load access ticket idempotency record", err, nil)
		}
		if found {
			return resolveExistingIdempotentResult(existing, fingerprint)
		}
	}

	file, err := h.repo.GetByID(ctx, command.FileID)
	if err != nil {
		return CreateAccessTicketResult{}, err
	}
	if err := file.EnsureReadable(command.Auth); err != nil {
		return CreateAccessTicketResult{}, err
	}
	policy, err := h.policies.GetByTenantID(ctx, file.TenantID)
	if err != nil {
		return CreateAccessTicketResult{}, err
	}
	if err := policy.EnsureAllowed(disposition); err != nil {
		return CreateAccessTicketResult{}, err
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

	result := CreateAccessTicketResult{
		Ticket:      ticket,
		RedirectURL: buildRedirectURL(h.redirectBasePath, ticket),
		ExpiresAt:   expiresAt,
	}
	if storageKey == "" {
		return result, nil
	}

	record := ports.AccessTicketIssueRecord{
		Fingerprint: fingerprint,
		Ticket:      result.Ticket,
		RedirectURL: result.RedirectURL,
		ExpiresAt:   result.ExpiresAt,
	}
	stored, err := h.idempotencyStore.PutIfAbsent(ctx, storageKey, record, ttl)
	if err != nil {
		return CreateAccessTicketResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "persist access ticket idempotency record", err, nil)
	}
	if stored {
		return result, nil
	}

	existing, found, err := h.idempotencyStore.Get(ctx, storageKey)
	if err != nil {
		return CreateAccessTicketResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "reload access ticket idempotency record", err, nil)
	}
	if !found {
		return CreateAccessTicketResult{}, xerrors.New(xerrors.CodeServiceUnavailable, "access ticket idempotency record was not found after conflict", nil)
	}

	return resolveExistingIdempotentResult(existing, fingerprint)
}

func buildRedirectURL(basePath, ticket string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	if basePath == "" {
		basePath = "/api/v1/access-tickets"
	}

	return fmt.Sprintf("%s/%s/redirect", basePath, ticket)
}

func resolveExistingIdempotentResult(record ports.AccessTicketIssueRecord, fingerprint string) (CreateAccessTicketResult, error) {
	if record.Fingerprint != fingerprint {
		return CreateAccessTicketResult{}, xerrors.New(xerrors.CodeConflict, "idempotency key is already used by another access ticket request", xerrors.Details{
			"field": "Idempotency-Key",
		})
	}

	return CreateAccessTicketResult{
		Ticket:      record.Ticket,
		RedirectURL: record.RedirectURL,
		ExpiresAt:   record.ExpiresAt,
	}, nil
}

func buildIdempotencyStorageKey(auth domain.AuthContext, fileID, idempotencyKey string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(auth.TenantID),
		strings.TrimSpace(auth.SubjectType),
		strings.TrimSpace(auth.SubjectID),
		strings.TrimSpace(fileID),
		strings.TrimSpace(idempotencyKey),
	}, "\n")))
	return hex.EncodeToString(sum[:])
}

func buildIdempotencyFingerprint(
	fileID string,
	auth domain.AuthContext,
	disposition domain.DownloadDisposition,
	responseName string,
	ttl time.Duration,
) (string, error) {
	payload, err := json.Marshal(struct {
		FileID       string `json:"fileId"`
		TenantID     string `json:"tenantId"`
		SubjectID    string `json:"subjectId"`
		SubjectType  string `json:"subjectType,omitempty"`
		Disposition  string `json:"disposition"`
		ResponseName string `json:"responseName,omitempty"`
		TTLNanos     int64  `json:"ttlNanos"`
	}{
		FileID:       fileID,
		TenantID:     auth.TenantID,
		SubjectID:    auth.SubjectID,
		SubjectType:  auth.SubjectType,
		Disposition:  string(disposition),
		ResponseName: responseName,
		TTLNanos:     ttl.Nanoseconds(),
	})
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
