package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestCreateTenantInitializesPolicyUsageAndAudit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	tenants := &stubTenantRepository{}
	policies := &stubTenantPolicyRepository{}
	usages := &stubTenantUsageRepository{}
	audits := &stubAuditLogRepository{}
	tx := &stubTxManager{}
	handler := commands.NewCreateTenantHandler(
		tenants,
		policies,
		usages,
		audits,
		tx,
		&stubIDGenerator{id: "audit-1"},
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), commands.CreateTenantCommand{
		TenantID:       "tenant-a",
		TenantName:     "Tenant A",
		ContactEmail:   "ops@example.com",
		Description:    "demo tenant",
		IdempotencyKey: "idem-1",
		Auth:           mustAdminContext(t, domain.RoleSuper, "tenant-a"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if tx.calls != 1 {
		t.Fatalf("tx calls = %d, want 1", tx.calls)
	}
	if tenants.created.TenantID != "tenant-a" || tenants.created.Status != domain.TenantStatusActive {
		t.Fatalf("unexpected created tenant: %#v", tenants.created)
	}
	if len(policies.createdDefaults) != 1 || policies.createdDefaults[0] != "tenant-a" {
		t.Fatalf("unexpected default policy initialization: %#v", policies.createdDefaults)
	}
	if len(usages.initialized) != 1 || usages.initialized[0] != "tenant-a" {
		t.Fatalf("unexpected usage initialization: %#v", usages.initialized)
	}
	if len(audits.records) != 1 || audits.records[0].Action != "tenant.create" {
		t.Fatalf("unexpected audit records: %#v", audits.records)
	}
	if result.TenantID != "tenant-a" || result.TenantName != "Tenant A" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestPatchTenantReturnsNotFoundWhenRepositoryMisses(t *testing.T) {
	t.Parallel()

	handler := commands.NewPatchTenantHandler(
		&stubTenantRepository{},
		&stubAuditLogRepository{},
		&stubTxManager{},
		&stubIDGenerator{id: "audit-2"},
		clock.NewFixed(time.Date(2026, 3, 22, 12, 10, 0, 0, time.UTC)),
	)

	_, err := handler.Handle(context.Background(), commands.PatchTenantCommand{
		TenantID: "tenant-a",
		Patch: domain.TenantPatch{
			Description: stringPtr("updated"),
		},
		Auth: mustAdminContext(t, domain.RoleGovernance, "tenant-a"),
	})
	if code := xerrors.CodeOf(err); code != domain.CodeTenantNotFound {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, domain.CodeTenantNotFound, err)
	}
}

func TestPatchTenantWritesAuditOnSuccess(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 12, 20, 0, 0, time.UTC)
	tenants := &stubTenantRepository{
		patched: &domain.Tenant{
			TenantID:     "tenant-a",
			TenantName:   "Tenant A CN",
			Status:       domain.TenantStatusActive,
			ContactEmail: "platform@example.com",
			Description:  "updated",
			CreatedAt:    now.Add(-time.Hour),
			UpdatedAt:    now,
		},
	}
	audits := &stubAuditLogRepository{}
	handler := commands.NewPatchTenantHandler(
		tenants,
		audits,
		&stubTxManager{},
		&stubIDGenerator{id: "audit-3"},
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), commands.PatchTenantCommand{
		TenantID: "tenant-a",
		Patch: domain.TenantPatch{
			TenantName:   stringPtr("Tenant A CN"),
			ContactEmail: stringPtr("platform@example.com"),
			Description:  stringPtr("updated"),
		},
		IdempotencyKey: "idem-2",
		Auth:           mustAdminContext(t, domain.RoleGovernance, "tenant-a"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(audits.records) != 1 {
		t.Fatalf("audit records = %d, want 1", len(audits.records))
	}
	if audits.records[0].Details["tenantName"] != "Tenant A CN" {
		t.Fatalf("unexpected audit details: %#v", audits.records[0].Details)
	}
	if result.ContactEmail != "platform@example.com" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestPatchTenantPolicyEnforcesDestructiveReasonAndWritesAudit(t *testing.T) {
	t.Parallel()

	current := &ports.TenantPolicyView{
		TenantID: "tenant-a",
		Policy: domain.TenantPolicy{
			MaxStorageBytes: int64Ptr(100),
		},
	}
	policies := &stubTenantPolicyRepository{
		current: current,
		patched: &ports.TenantPolicyView{
			TenantID: "tenant-a",
			Policy: domain.TenantPolicy{
				MaxStorageBytes: int64Ptr(80),
			},
		},
	}
	handler := commands.NewPatchTenantPolicyHandler(
		policies,
		&stubAuditLogRepository{},
		&stubTxManager{},
		&stubIDGenerator{id: "audit-4"},
		clock.NewFixed(time.Date(2026, 3, 22, 12, 30, 0, 0, time.UTC)),
	)

	_, err := handler.Handle(context.Background(), commands.PatchTenantPolicyCommand{
		TenantID: "tenant-a",
		Patch: domain.TenantPolicyPatch{
			MaxStorageBytes: int64Ptr(80),
		},
		Auth: mustAdminContext(t, domain.RoleGovernance, "tenant-a"),
	})
	if code := xerrors.CodeOf(err); code != xerrors.CodeInvalidArgument {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeInvalidArgument, err)
	}

	audits := &stubAuditLogRepository{}
	handler = commands.NewPatchTenantPolicyHandler(
		policies,
		audits,
		&stubTxManager{},
		&stubIDGenerator{id: "audit-5"},
		clock.NewFixed(time.Date(2026, 3, 22, 12, 31, 0, 0, time.UTC)),
	)
	result, err := handler.Handle(context.Background(), commands.PatchTenantPolicyCommand{
		TenantID: "tenant-a",
		Patch: domain.TenantPolicyPatch{
			MaxStorageBytes: int64Ptr(80),
			Reason:          "tighten quota",
		},
		Auth: mustAdminContext(t, domain.RoleGovernance, "tenant-a"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(audits.records) != 1 || audits.records[0].Action != "tenant_policy.patch" {
		t.Fatalf("unexpected audit records: %#v", audits.records)
	}
	if result.MaxStorageBytes == nil || *result.MaxStorageBytes != 80 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

type stubTenantRepository struct {
	created   domain.Tenant
	createErr error
	patched   *domain.Tenant
	patchErr  error
}

func (s *stubTenantRepository) Create(_ context.Context, tenant domain.Tenant) error {
	s.created = tenant
	return s.createErr
}

func (s *stubTenantRepository) GetByID(context.Context, string) (*domain.Tenant, error) {
	panic("unexpected call")
}

func (s *stubTenantRepository) List(context.Context, ports.ListTenantsQuery) ([]domain.Tenant, string, error) {
	panic("unexpected call")
}

func (s *stubTenantRepository) Patch(_ context.Context, _ string, _ domain.TenantPatch) (*domain.Tenant, error) {
	return s.patched, s.patchErr
}

type stubTenantPolicyRepository struct {
	createdDefaults []string
	createErr       error
	current         *ports.TenantPolicyView
	getErr          error
	patched         *ports.TenantPolicyView
	patchErr        error
}

func (s *stubTenantPolicyRepository) CreateDefault(_ context.Context, tenantID string) error {
	s.createdDefaults = append(s.createdDefaults, tenantID)
	return s.createErr
}

func (s *stubTenantPolicyRepository) GetByTenantID(context.Context, string) (*ports.TenantPolicyView, error) {
	return s.current, s.getErr
}

func (s *stubTenantPolicyRepository) Patch(context.Context, string, domain.TenantPolicyPatch) (*ports.TenantPolicyView, error) {
	return s.patched, s.patchErr
}

type stubTenantUsageRepository struct {
	initialized []string
	initErr     error
}

func (s *stubTenantUsageRepository) Initialize(_ context.Context, tenantID string) error {
	s.initialized = append(s.initialized, tenantID)
	return s.initErr
}

func (s *stubTenantUsageRepository) GetByTenantID(context.Context, string) (*ports.TenantUsageView, error) {
	panic("unexpected call")
}

type stubAuditLogRepository struct {
	records []ports.AuditLogRecord
	err     error
}

func (s *stubAuditLogRepository) Append(_ context.Context, record ports.AuditLogRecord) error {
	s.records = append(s.records, record)
	return s.err
}

func (s *stubAuditLogRepository) List(context.Context, ports.ListAuditLogsQuery) ([]ports.AuditLogRecord, string, error) {
	panic("unexpected call")
}

type stubTxManager struct {
	calls int
}

func (s *stubTxManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	s.calls++
	return fn(ctx)
}

type stubIDGenerator struct {
	id string
}

func (s *stubIDGenerator) New() (string, error) {
	return s.id, nil
}

func mustAdminContext(t *testing.T, role domain.Role, scope string) domain.AdminContext {
	t.Helper()

	ctx, err := domain.NewAdminContext(domain.AdminContextInput{
		AdminID:      "admin-1",
		Roles:        []string{string(role)},
		TenantScopes: []string{scope},
		RequestID:    "req-1",
	})
	if err != nil {
		t.Fatalf("NewAdminContext() error = %v", err)
	}

	return ctx
}

func stringPtr(value string) *string {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}
