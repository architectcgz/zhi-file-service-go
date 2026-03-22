package commands

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
)

type CreateTenantCommand struct {
	TenantID       string
	TenantName     string
	ContactEmail   string
	Description    string
	IdempotencyKey string
	Auth           domain.AdminContext
}

type CreateTenantHandler struct {
	tenants  ports.TenantRepository
	policies ports.TenantPolicyRepository
	usages   ports.TenantUsageRepository
	audits   ports.AuditLogRepository
	tx       ports.TxManager
	idgen    ids.Generator
	clock    clock.Clock
}

func NewCreateTenantHandler(
	tenants ports.TenantRepository,
	policies ports.TenantPolicyRepository,
	usages ports.TenantUsageRepository,
	audits ports.AuditLogRepository,
	tx ports.TxManager,
	idgen ids.Generator,
	clk clock.Clock,
) CreateTenantHandler {
	return CreateTenantHandler{
		tenants:  tenants,
		policies: policies,
		usages:   usages,
		audits:   audits,
		tx:       defaultTxManager(tx),
		idgen:    defaultIDGenerator(idgen),
		clock:    defaultClock(clk),
	}
}

func (h CreateTenantHandler) Handle(ctx context.Context, command CreateTenantCommand) (view.Tenant, error) {
	tenantID, err := requiredField(command.TenantID, "tenantId")
	if err != nil {
		return view.Tenant{}, err
	}
	tenantName, err := requiredField(command.TenantName, "tenantName")
	if err != nil {
		return view.Tenant{}, err
	}
	if err := validateOptionalEmail(command.ContactEmail); err != nil {
		return view.Tenant{}, err
	}
	if err := newGuard().EnsureCreateTenant(command.Auth, tenantID); err != nil {
		return view.Tenant{}, err
	}

	now := h.clock.Now()
	tenant := domain.Tenant{
		TenantID:     tenantID,
		TenantName:   tenantName,
		Status:       domain.TenantStatusActive,
		ContactEmail: optionalField(command.ContactEmail),
		Description:  optionalField(command.Description),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.tenants.Create(txCtx, tenant); err != nil {
			return err
		}
		if err := h.policies.CreateDefault(txCtx, tenantID); err != nil {
			return err
		}
		if err := h.usages.Initialize(txCtx, tenantID); err != nil {
			return err
		}

		record, err := newAuditRecord(h.idgen, h.clock, command.Auth, tenantID, actionTenantCreate, "tenant", tenantID, map[string]any{
			"tenantName":     tenantName,
			"status":         tenant.Status,
			"idempotencyKey": optionalField(command.IdempotencyKey),
		})
		if err != nil {
			return err
		}

		return h.audits.Append(txCtx, record)
	}); err != nil {
		return view.Tenant{}, err
	}

	return view.FromTenant(tenant), nil
}
