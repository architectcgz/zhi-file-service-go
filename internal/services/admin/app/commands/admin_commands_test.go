package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
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

func TestPatchTenantRequiresReasonForDestructiveStatus(t *testing.T) {
	t.Parallel()

	status := domain.TenantStatusSuspended
	tenants := &stubTenantRepository{}
	audits := &stubAuditLogRepository{}
	tx := &stubTxManager{}
	handler := commands.NewPatchTenantHandler(
		tenants,
		audits,
		tx,
		&stubIDGenerator{id: "audit-destructive"},
		clock.NewFixed(time.Date(2026, 3, 22, 12, 25, 0, 0, time.UTC)),
	)

	_, err := handler.Handle(context.Background(), commands.PatchTenantCommand{
		TenantID: "tenant-a",
		Patch: domain.TenantPatch{
			Status: &status,
		},
		Auth: mustAdminContext(t, domain.RoleGovernance, "tenant-a"),
	})
	if code := xerrors.CodeOf(err); code != xerrors.CodeInvalidArgument {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeInvalidArgument, err)
	}
	if tenants.patchCalls != 0 {
		t.Fatalf("patchCalls = %d, want 0", tenants.patchCalls)
	}
	if tx.calls != 0 {
		t.Fatalf("tx calls = %d, want 0", tx.calls)
	}
	if len(audits.records) != 0 {
		t.Fatalf("unexpected audit records: %#v", audits.records)
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

func TestDeleteFileWritesAuditAndOutbox(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 12, 40, 0, 0, time.UTC)
	files := &stubAdminFileRepository{
		file: &ports.AdminFileView{
			FileID:      "file-1",
			TenantID:    "tenant-a",
			BlobID:      "blob-1",
			FileName:    "report.pdf",
			ContentType: "application/pdf",
			SizeBytes:   10,
			AccessLevel: pkgstorage.AccessLevelPrivate,
			Status:      "ACTIVE",
		},
		deleted: &ports.DeleteFileRecord{
			File: ports.AdminFileView{
				FileID:      "file-1",
				TenantID:    "tenant-a",
				BlobID:      "blob-1",
				FileName:    "report.pdf",
				ContentType: "application/pdf",
				SizeBytes:   10,
				AccessLevel: pkgstorage.AccessLevelPrivate,
				Status:      "DELETED",
				DeletedAt:   timePtr(now),
			},
			PhysicalDeleteScheduled: true,
		},
	}
	audits := &stubAuditLogRepository{}
	outbox := &stubOutboxPublisher{}
	handler := commands.NewDeleteFileHandler(files, audits, outbox, &stubTxManager{}, &stubIDGenerator{id: "audit-6"}, clock.NewFixed(now))

	result, err := handler.Handle(context.Background(), commands.DeleteFileCommand{
		FileID:         "file-1",
		Reason:         "manual cleanup",
		IdempotencyKey: "idem-3",
		Auth:           mustAdminContext(t, domain.RoleGovernance, "tenant-a"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(audits.records) != 1 || audits.records[0].Action != "file.delete" {
		t.Fatalf("unexpected audit records: %#v", audits.records)
	}
	if len(outbox.events) != 1 || outbox.events[0].EventType != "file.asset.delete_requested.v1" {
		t.Fatalf("unexpected outbox events: %#v", outbox.events)
	}
	if files.lastDelete != now {
		t.Fatalf("lastDelete = %v, want %v", files.lastDelete, now)
	}
	if got := audits.records[0].Details["reason"]; got != "manual cleanup" {
		t.Fatalf("audit reason = %#v, want %q", got, "manual cleanup")
	}
	if got := audits.records[0].Details["idempotencyKey"]; got != "idem-3" {
		t.Fatalf("audit idempotencyKey = %#v, want %q", got, "idem-3")
	}
	if got := audits.records[0].Details["blobId"]; got != "blob-1" {
		t.Fatalf("audit blobId = %#v, want %q", got, "blob-1")
	}
	event := outbox.events[0]
	if event.AggregateType != "file_asset" || event.AggregateID != "file-1" {
		t.Fatalf("unexpected aggregate info: %#v", event)
	}
	if event.RequestID != "req-1" || event.TenantID != "tenant-a" {
		t.Fatalf("unexpected event routing info: %#v", event)
	}
	if got := event.Payload["blobObjectId"]; got != "blob-1" {
		t.Fatalf("payload blobObjectId = %#v, want %q", got, "blob-1")
	}
	if got := event.Payload["deletedBy"]; got != "admin-1" {
		t.Fatalf("payload deletedBy = %#v, want %q", got, "admin-1")
	}
	if got := event.Payload["reason"]; got != "manual cleanup" {
		t.Fatalf("payload reason = %#v, want %q", got, "manual cleanup")
	}
	if got := event.Payload["occurredAt"]; got != now.UTC().Format(time.RFC3339) {
		t.Fatalf("payload occurredAt = %#v, want %q", got, now.UTC().Format(time.RFC3339))
	}
	if result.FileID != "file-1" || !result.PhysicalDeleteScheduled {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestDeleteFileRequiresReasonBeforeDelete(t *testing.T) {
	t.Parallel()

	files := &stubAdminFileRepository{
		file: &ports.AdminFileView{
			FileID:   "file-1",
			TenantID: "tenant-a",
			Status:   "ACTIVE",
		},
	}
	audits := &stubAuditLogRepository{}
	outbox := &stubOutboxPublisher{}
	tx := &stubTxManager{}
	handler := commands.NewDeleteFileHandler(
		files,
		audits,
		outbox,
		tx,
		&stubIDGenerator{id: "audit-reason"},
		clock.NewFixed(time.Date(2026, 3, 22, 12, 42, 0, 0, time.UTC)),
	)

	_, err := handler.Handle(context.Background(), commands.DeleteFileCommand{
		FileID: "file-1",
		Reason: "   ",
		Auth:   mustAdminContext(t, domain.RoleGovernance, "tenant-a"),
	})
	if code := xerrors.CodeOf(err); code != xerrors.CodeInvalidArgument {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeInvalidArgument, err)
	}
	if files.markDeleteCalls != 0 {
		t.Fatalf("markDeleteCalls = %d, want 0", files.markDeleteCalls)
	}
	if tx.calls != 0 {
		t.Fatalf("tx calls = %d, want 0", tx.calls)
	}
	if len(audits.records) != 0 {
		t.Fatalf("unexpected audit records: %#v", audits.records)
	}
	if len(outbox.events) != 0 {
		t.Fatalf("unexpected outbox events: %#v", outbox.events)
	}
}

func TestDeleteFileRejectsTenantOutOfScope(t *testing.T) {
	t.Parallel()

	files := &stubAdminFileRepository{
		file: &ports.AdminFileView{
			FileID:   "file-1",
			TenantID: "tenant-b",
			Status:   "ACTIVE",
		},
	}
	handler := commands.NewDeleteFileHandler(
		files,
		&stubAuditLogRepository{},
		&stubOutboxPublisher{},
		&stubTxManager{},
		&stubIDGenerator{id: "audit-scope"},
		clock.NewFixed(time.Date(2026, 3, 22, 12, 43, 0, 0, time.UTC)),
	)

	_, err := handler.Handle(context.Background(), commands.DeleteFileCommand{
		FileID: "file-1",
		Reason: "manual cleanup",
		Auth:   mustAdminContext(t, domain.RoleGovernance, "tenant-a"),
	})
	if code := xerrors.CodeOf(err); code != domain.CodeTenantScopeDenied {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, domain.CodeTenantScopeDenied, err)
	}
	if files.markDeleteCalls != 0 {
		t.Fatalf("markDeleteCalls = %d, want 0", files.markDeleteCalls)
	}
}

func TestDeleteFileIsIdempotentWhenAlreadyDeleted(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 12, 45, 0, 0, time.UTC)
	files := &stubAdminFileRepository{
		file: &ports.AdminFileView{
			FileID:   "file-1",
			TenantID: "tenant-a",
			Status:   "DELETED",
		},
		deleted: &ports.DeleteFileRecord{
			File: ports.AdminFileView{
				FileID:    "file-1",
				TenantID:  "tenant-a",
				Status:    "DELETED",
				DeletedAt: timePtr(now.Add(-time.Minute)),
			},
			PhysicalDeleteScheduled: true,
			AlreadyDeleted:          true,
		},
	}
	audits := &stubAuditLogRepository{}
	outbox := &stubOutboxPublisher{}
	handler := commands.NewDeleteFileHandler(files, audits, outbox, &stubTxManager{}, &stubIDGenerator{id: "audit-7"}, clock.NewFixed(now))

	result, err := handler.Handle(context.Background(), commands.DeleteFileCommand{
		FileID: "file-1",
		Reason: "repeat request",
		Auth:   mustAdminContext(t, domain.RoleGovernance, "tenant-a"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(audits.records) != 0 {
		t.Fatalf("expected no audit records, got %#v", audits.records)
	}
	if len(outbox.events) != 0 {
		t.Fatalf("expected no outbox events, got %#v", outbox.events)
	}
	if result.Status != "DELETED" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

type stubTenantRepository struct {
	created   domain.Tenant
	createErr error
	patched   *domain.Tenant
	patchErr  error
	patchCalls int
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
	s.patchCalls++
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

type stubAdminFileRepository struct {
	file       *ports.AdminFileView
	deleted    *ports.DeleteFileRecord
	err        error
	lastDelete time.Time
	markDeleteCalls int
}

func (s *stubAdminFileRepository) GetByID(context.Context, string) (*ports.AdminFileView, error) {
	return s.file, s.err
}

func (s *stubAdminFileRepository) List(context.Context, ports.ListFilesQuery) ([]ports.AdminFileView, string, error) {
	panic("unexpected call")
}

func (s *stubAdminFileRepository) MarkDeleted(_ context.Context, _ string, deletedAt time.Time) (*ports.DeleteFileRecord, error) {
	s.markDeleteCalls++
	s.lastDelete = deletedAt
	return s.deleted, s.err
}

type stubOutboxPublisher struct {
	events []ports.OutboxEvent
	err    error
}

func (s *stubOutboxPublisher) Publish(_ context.Context, event ports.OutboxEvent) error {
	s.events = append(s.events, event)
	return s.err
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

func timePtr(value time.Time) *time.Time {
	return &value
}
