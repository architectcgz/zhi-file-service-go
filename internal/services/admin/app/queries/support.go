package queries

import (
	"slices"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

func authorizeOperation(ctx domain.AdminContext, operation domain.Operation) error {
	return domain.AuthorizeOperation(ctx, operation)
}

func authorizeTenantOperation(ctx domain.AdminContext, operation domain.Operation, tenantID string) error {
	if err := domain.AuthorizeOperation(ctx, operation); err != nil {
		return err
	}

	return domain.EnsureTenantScope(ctx, tenantID)
}

func scopedTenants(ctx domain.AdminContext) []string {
	if ctx.IsGlobalScope() {
		return nil
	}

	return slices.Clone(ctx.TenantScopes)
}

func normalizeListConfig(defaultLimit int, maxLimit int) (int, int) {
	if defaultLimit <= 0 {
		defaultLimit = defaultListLimit
	}
	if maxLimit <= 0 {
		maxLimit = maxListLimit
	}
	if maxLimit < defaultLimit {
		maxLimit = defaultLimit
	}

	return defaultLimit, maxLimit
}

func normalizeOptionalQueryValue(value string, field string) (string, error) {
	normalized := strings.TrimSpace(value)
	if value != "" && normalized == "" {
		return "", xerrors.New(xerrors.CodeInvalidArgument, "query parameter is invalid", xerrors.Details{
			"field":  field,
			"reason": "must_not_be_blank",
		})
	}

	return normalized, nil
}

func normalizeOptionalAuditQueryValue(value string, field string) (string, error) {
	normalized := strings.TrimSpace(value)
	if value != "" && normalized == "" {
		return "", domain.ErrAuditQueryInvalid(field, "must_not_be_blank")
	}

	return normalized, nil
}

func normalizeFileStatus(value string) (string, error) {
	status, err := normalizeOptionalQueryValue(value, "status")
	if err != nil || status == "" {
		return status, err
	}

	switch status {
	case "ACTIVE", "DELETED":
		return status, nil
	default:
		return "", xerrors.New(xerrors.CodeInvalidArgument, "query parameter is invalid", xerrors.Details{
			"field":  "status",
			"reason": "unsupported_value",
		})
	}
}
