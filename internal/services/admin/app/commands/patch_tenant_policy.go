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

type PatchTenantPolicyCommand struct {
	TenantID       string
	Patch          domain.TenantPolicyPatch
	IdempotencyKey string
	Auth           domain.AdminContext
}

type PatchTenantPolicyHandler struct {
	policies ports.TenantPolicyRepository
	audits   ports.AuditLogRepository
	tx       ports.TxManager
	idgen    ids.Generator
	clock    clock.Clock
}

func NewPatchTenantPolicyHandler(
	policies ports.TenantPolicyRepository,
	audits ports.AuditLogRepository,
	tx ports.TxManager,
	idgen ids.Generator,
	clk clock.Clock,
) PatchTenantPolicyHandler {
	return PatchTenantPolicyHandler{
		policies: policies,
		audits:   audits,
		tx:       defaultTxManager(tx),
		idgen:    defaultIDGenerator(idgen),
		clock:    defaultClock(clk),
	}
}

func (h PatchTenantPolicyHandler) Handle(ctx context.Context, command PatchTenantPolicyCommand) (view.TenantPolicy, error) {
	tenantID, err := requiredField(command.TenantID, "tenantId")
	if err != nil {
		return view.TenantPolicy{}, err
	}
	patch := command.Patch.Normalize()
	if patch.IsEmpty() {
		return view.TenantPolicy{}, xerrors.New(xerrors.CodeInvalidArgument, "tenant policy patch is empty", xerrors.Details{
			"field": "body",
		})
	}
	if err := patch.Validate(); err != nil {
		return view.TenantPolicy{}, err
	}

	var updated *ports.TenantPolicyView
	if err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := h.policies.GetByTenantID(txCtx, tenantID)
		if err != nil {
			return err
		}
		if current == nil {
			return domain.ErrTenantNotFound(tenantID)
		}
		if err := newGuard().EnsureTenantPolicyPatch(command.Auth, tenantID, current.Policy.Normalize(), patch); err != nil {
			return err
		}

		updated, err = h.policies.Patch(txCtx, tenantID, patch)
		if err != nil {
			return err
		}
		if updated == nil {
			return domain.ErrTenantNotFound(tenantID)
		}

		record, err := newAuditRecord(txCtx, h.idgen, h.clock, command.Auth, tenantID, actionTenantPolicyPatch, tenantPolicyPatchDetails(patch, command.IdempotencyKey))
		if err != nil {
			return err
		}

		return h.audits.Append(txCtx, record)
	}); err != nil {
		return view.TenantPolicy{}, err
	}

	return view.FromTenantPolicy(updated), nil
}

func tenantPolicyPatchDetails(patch domain.TenantPolicyPatch, idempotencyKey string) map[string]any {
	details := map[string]any{
		"idempotencyKey": optionalField(idempotencyKey),
	}
	if patch.MaxStorageBytes != nil {
		details["maxStorageBytes"] = *patch.MaxStorageBytes
	}
	if patch.MaxFileCount != nil {
		details["maxFileCount"] = *patch.MaxFileCount
	}
	if patch.MaxSingleFileSize != nil {
		details["maxSingleFileSize"] = *patch.MaxSingleFileSize
	}
	if patch.AllowedMimeTypes != nil {
		details["allowedMimeTypes"] = patch.AllowedMimeTypes
	}
	if patch.AllowedExtensions != nil {
		details["allowedExtensions"] = patch.AllowedExtensions
	}
	if patch.DefaultAccessLevel != nil {
		details["defaultAccessLevel"] = *patch.DefaultAccessLevel
	}
	if patch.AutoCreateEnabled != nil {
		details["autoCreateEnabled"] = *patch.AutoCreateEnabled
	}
	if patch.Reason != "" {
		details["reason"] = patch.Reason
	}

	return details
}
