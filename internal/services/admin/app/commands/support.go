package commands

import (
	"context"
	"strings"

	appcore "github.com/architectcgz/zhi-file-service-go/internal/services/admin/app"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const (
	actionTenantCreate      = "tenant.create"
	actionTenantPatch       = "tenant.patch"
	actionTenantPolicyPatch = "tenant_policy.patch"
	actionFileDelete        = "file.delete"
)

type inlineTxManager struct{}

func (inlineTxManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func defaultTxManager(tx ports.TxManager) ports.TxManager {
	if tx == nil {
		return inlineTxManager{}
	}

	return tx
}

func defaultClock(value clock.Clock) clock.Clock {
	if value == nil {
		return clock.SystemClock{}
	}

	return value
}

func defaultIDGenerator(value ids.Generator) ids.Generator {
	if value == nil {
		return ids.NewGenerator(nil, nil)
	}

	return value
}

func newGuard() appcore.Guard {
	return appcore.NewGuard()
}

func requiredField(value string, field string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized != "" {
		return normalized, nil
	}

	return "", xerrors.New(xerrors.CodeInvalidArgument, "field is required", xerrors.Details{
		"field": field,
	})
}

func optionalField(value string) string {
	return strings.TrimSpace(value)
}

func validateOptionalEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" || strings.Contains(email, "@") {
		return nil
	}

	return xerrors.New(xerrors.CodeInvalidArgument, "contact email is invalid", xerrors.Details{
		"field": "contactEmail",
	})
}

func newAuditRecord(
	idgen ids.Generator,
	clk clock.Clock,
	auth domain.AdminContext,
	tenantID string,
	action string,
	targetType string,
	targetID string,
	details map[string]any,
) (ports.AuditLogRecord, error) {
	auditID, err := idgen.New()
	if err != nil {
		return ports.AuditLogRecord{}, xerrors.Wrap(xerrors.CodeInternalError, "generate audit log id", err, nil)
	}

	return ports.AuditLogRecord{
		AuditID:      auditID,
		AdminSubject: auth.AdminID,
		TenantID:     strings.TrimSpace(tenantID),
		Action:       action,
		TargetType:   strings.TrimSpace(targetType),
		TargetID:     strings.TrimSpace(targetID),
		RequestID:    auth.RequestID,
		Details:      details,
		CreatedAt:    clk.Now(),
	}, nil
}
