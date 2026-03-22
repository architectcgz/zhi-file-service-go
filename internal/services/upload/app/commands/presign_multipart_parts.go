package commands

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type PresignMultipartPart struct {
	PartNumber int
}

type PresignMultipartPartsCommand struct {
	UploadSessionID string
	ExpiresIn       time.Duration
	Parts           []PresignMultipartPart
	Auth            domain.AuthContext
}

type PresignMultipartPartsConfig struct {
	DefaultTTL time.Duration
	MaxTTL     time.Duration
}

type PresignMultipartPartsResult struct {
	Parts []view.PresignedPart
}

type PresignMultipartPartsHandler struct {
	sessions   ports.SessionRepository
	presign    ports.PresignManager
	clock      clock.Clock
	defaultTTL time.Duration
	maxTTL     time.Duration
}

func NewPresignMultipartPartsHandler(
	sessions ports.SessionRepository,
	presign ports.PresignManager,
	clk clock.Clock,
	cfg PresignMultipartPartsConfig,
) PresignMultipartPartsHandler {
	if clk == nil {
		clk = clock.SystemClock{}
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 15 * time.Minute
	}
	if cfg.MaxTTL <= 0 {
		cfg.MaxTTL = 24 * time.Hour
	}

	return PresignMultipartPartsHandler{
		sessions:   sessions,
		presign:    presign,
		clock:      clk,
		defaultTTL: cfg.DefaultTTL,
		maxTTL:     cfg.MaxTTL,
	}
}

func (h PresignMultipartPartsHandler) Handle(ctx context.Context, command PresignMultipartPartsCommand) (PresignMultipartPartsResult, error) {
	if err := command.Auth.RequireFileWrite(); err != nil {
		return PresignMultipartPartsResult{}, err
	}
	if strings.TrimSpace(command.UploadSessionID) == "" {
		return PresignMultipartPartsResult{}, xerrors.New(xerrors.CodeInvalidArgument, "upload session id is required", xerrors.Details{
			"field": "uploadSessionId",
		})
	}

	partNumbers, err := validatePresignPartNumbers(command.Parts)
	if err != nil {
		return PresignMultipartPartsResult{}, err
	}
	ttl := normalizePresignTTL(command.ExpiresIn, h.defaultTTL, h.maxTTL)

	session, err := h.sessions.GetByID(ctx, command.Auth.TenantID, command.UploadSessionID)
	if err != nil {
		return PresignMultipartPartsResult{}, err
	}
	if err := command.Auth.EnsureSessionAccess(session); err != nil {
		return PresignMultipartPartsResult{}, err
	}
	if session.Mode != domain.SessionModeDirect {
		return PresignMultipartPartsResult{}, newUploadModeMismatch(session.Mode, domain.SessionModeDirect)
	}
	if session.Status != domain.SessionStatusInitiated && session.Status != domain.SessionStatusUploading {
		return PresignMultipartPartsResult{}, newUploadStateConflict(session.Status, domain.SessionStatusInitiated, domain.SessionStatusUploading)
	}
	for _, partNumber := range partNumbers {
		if session.TotalParts > 0 && partNumber > session.TotalParts {
			return PresignMultipartPartsResult{}, xerrors.New(xerrors.CodeInvalidArgument, "part number exceeds upload session total parts", xerrors.Details{
				"field":      "parts.partNumber",
				"partNumber": partNumber,
				"totalParts": session.TotalParts,
			})
		}
	}

	now := h.clock.Now()
	result := PresignMultipartPartsResult{
		Parts: make([]view.PresignedPart, 0, len(partNumbers)),
	}
	expiresAt := now.Add(ttl)
	for _, partNumber := range partNumbers {
		url, headers, err := h.presign.PresignUploadPart(ctx, session.Object, session.ProviderUploadID, partNumber, ttl)
		if err != nil {
			return PresignMultipartPartsResult{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "presign multipart part", err, xerrors.Details{
				"partNumber": partNumber,
			})
		}
		result.Parts = append(result.Parts, view.PresignedPart{
			PartNumber: partNumber,
			URL:        url,
			Headers:    view.CloneHeaders(headers),
			ExpiresAt:  expiresAt,
		})
	}

	return result, nil
}

func normalizePresignTTL(value time.Duration, defaultTTL time.Duration, maxTTL time.Duration) time.Duration {
	if value <= 0 {
		return defaultTTL
	}
	if value > maxTTL {
		return maxTTL
	}

	return value
}

func validatePresignPartNumbers(parts []PresignMultipartPart) ([]int, error) {
	if len(parts) == 0 {
		return nil, xerrors.New(xerrors.CodeInvalidArgument, "at least one part is required", xerrors.Details{
			"field": "parts",
		})
	}

	seen := make(map[int]struct{}, len(parts))
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		if part.PartNumber < 1 {
			return nil, xerrors.New(xerrors.CodeInvalidArgument, "part number must be >= 1", xerrors.Details{
				"field":      "parts.partNumber",
				"partNumber": part.PartNumber,
			})
		}
		if _, ok := seen[part.PartNumber]; ok {
			return nil, xerrors.New(xerrors.CodeInvalidArgument, "part number must be unique", xerrors.Details{
				"field":      "parts.partNumber",
				"partNumber": part.PartNumber,
			})
		}
		seen[part.PartNumber] = struct{}{}
		values = append(values, part.PartNumber)
	}

	slices.Sort(values)
	return values, nil
}

func newUploadModeMismatch(current domain.SessionMode, required domain.SessionMode) error {
	return xerrors.New(domain.CodeUploadModeInvalid, "upload mode does not support current operation", xerrors.Details{
		"currentMode":  string(current),
		"requiredMode": string(required),
	})
}

func newUploadStateConflict(current domain.SessionStatus, allowed ...domain.SessionStatus) error {
	values := make([]string, 0, len(allowed))
	for _, status := range allowed {
		values = append(values, string(status))
	}

	return xerrors.New(domain.CodeUploadSessionStateConflict, "upload session state conflict", xerrors.Details{
		"currentStatus": string(current),
		"allowedStatus": values,
	})
}
