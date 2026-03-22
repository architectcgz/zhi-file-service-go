package commands

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type PatchTenantCommand struct {
	TenantID       string
	Patch          domain.TenantPatch
	IdempotencyKey string
	Auth           domain.AdminContext
}

type PatchTenantHandler struct {
	tenants ports.TenantRepository
	audits  ports.AuditLogRepository
	tx      ports.TxManager
	idgen   ids.Generator
	clock   clock.Clock
}

func NewPatchTenantHandler(
	tenants ports.TenantRepository,
	audits ports.AuditLogRepository,
	tx ports.TxManager,
	idgen ids.Generator,
	clk clock.Clock,
) PatchTenantHandler {
	return PatchTenantHandler{
		tenants: tenants,
		audits:  audits,
		tx:      defaultTxManager(tx),
		idgen:   defaultIDGenerator(idgen),
		clock:   defaultClock(clk),
	}
}

func (h PatchTenantHandler) Handle(ctx context.Context, command PatchTenantCommand) (view.Tenant, error) {
	tenantID, err := requiredField(command.TenantID, "tenantId")
	if err != nil {
		return view.Tenant{}, err
	}
	patch := command.Patch.Normalize()
	if patch.IsEmpty() {
		return view.Tenant{}, xerrors.New(xerrors.CodeInvalidArgument, "tenant patch is empty", xerrors.Details{
			"field": "body",
		})
	}
	if patch.Status != nil {
		if err := patch.Status.Validate(); err != nil {
			return view.Tenant{}, err
		}
	}
	if patch.ContactEmail != nil {
		if err := validateOptionalEmail(*patch.ContactEmail); err != nil {
			return view.Tenant{}, err
		}
	}
	if err := newGuard().EnsureTenantPatch(command.Auth, tenantID, patch); err != nil {
		return view.Tenant{}, err
	}

	var updated *domain.Tenant
	if err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		updated, err = h.tenants.Patch(txCtx, tenantID, patch)
		if err != nil {
			return err
		}
		if updated == nil {
			return domain.ErrTenantNotFound(tenantID)
		}

		record, err := newAuditRecord(txCtx, h.idgen, h.clock, command.Auth, tenantID, actionTenantPatch, tenantPatchDetails(patch, command.IdempotencyKey))
		if err != nil {
			return err
		}

		return h.audits.Append(txCtx, record)
	}); err != nil {
		return view.Tenant{}, err
	}

	return view.FromTenant(*updated), nil
}

func tenantPatchDetails(patch domain.TenantPatch, idempotencyKey string) map[string]any {
	details := map[string]any{
		"idempotencyKey": optionalField(idempotencyKey),
	}
	if patch.TenantName != nil {
		details["tenantName"] = *patch.TenantName
	}
	if patch.Status != nil {
		details["status"] = *patch.Status
	}
	if patch.ContactEmail != nil {
		details["contactEmail"] = *patch.ContactEmail
	}
	if patch.Description != nil {
		details["description"] = *patch.Description
	}
	if patch.Reason != "" {
		details["reason"] = patch.Reason
	}

	return details
}
