package queries_test

import (
	"context"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
)

func TestGetTenantReturnsTenant(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 13, 0, 0, 0, time.UTC)
	handler := queries.NewGetTenantHandler(&stubQueryTenantRepository{
		tenant: &domain.Tenant{
			TenantID:     "tenant-a",
			TenantName:   "Tenant A",
			Status:       domain.TenantStatusActive,
			ContactEmail: "ops@example.com",
			Description:  "demo",
			CreatedAt:    now.Add(-time.Hour),
			UpdatedAt:    now,
		},
	})

	result, err := handler.Handle(context.Background(), queries.GetTenantQuery{
		TenantID: "tenant-a",
		Auth:     mustQueryAdminContext(t, domain.RoleReadonly, "tenant-a"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.TenantID != "tenant-a" || result.Status != domain.TenantStatusActive {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestListTenantsPassesScopeStatusAndNormalizedLimit(t *testing.T) {
	t.Parallel()

	status := domain.TenantStatusActive
	repo := &stubQueryTenantRepository{
		items: []domain.Tenant{
			{TenantID: "tenant-a", TenantName: "Tenant A", Status: status},
		},
		nextCursor: "cursor-2",
	}
	handler := queries.NewListTenantsHandler(repo, queries.ListTenantsConfig{
		DefaultLimit: 20,
		MaxLimit:     100,
	})

	result, err := handler.Handle(context.Background(), queries.ListTenantsQuery{
		Limit:  999,
		Status: &status,
		Auth:   mustQueryAdminContextWithScopes(t, domain.RoleReadonly, "tenant-a", "tenant-b"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repo.listQuery.Limit != 100 {
		t.Fatalf("normalized limit = %d, want 100", repo.listQuery.Limit)
	}
	if len(repo.listQuery.TenantScopes) != 2 {
		t.Fatalf("tenant scopes = %#v, want two scoped tenants", repo.listQuery.TenantScopes)
	}
	if result.NextCursor != "cursor-2" || len(result.Items) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGetTenantPolicyReturnsFlattenedPolicy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 13, 10, 0, 0, time.UTC)
	handler := queries.NewGetTenantPolicyHandler(&stubQueryTenantPolicyRepository{
		policy: &ports.TenantPolicyView{
			TenantID: "tenant-a",
			Policy: domain.TenantPolicy{
				MaxStorageBytes:    int64Ptr(1024),
				AllowedMimeTypes:   []string{" image/png ", "image/png"},
				DefaultAccessLevel: stringPtr("private"),
			},
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now,
		},
	})

	result, err := handler.Handle(context.Background(), queries.GetTenantPolicyQuery{
		TenantID: "tenant-a",
		Auth:     mustQueryAdminContext(t, domain.RoleReadonly, "tenant-a"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.DefaultAccessLevel == nil || *result.DefaultAccessLevel != "PRIVATE" {
		t.Fatalf("unexpected policy result: %#v", result)
	}
	if len(result.AllowedMimeTypes) != 1 || result.AllowedMimeTypes[0] != "image/png" {
		t.Fatalf("unexpected mime types: %#v", result.AllowedMimeTypes)
	}
}

func TestGetTenantUsageReturnsUsageView(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 13, 20, 0, 0, time.UTC)
	lastUpload := now.Add(-10 * time.Minute)
	handler := queries.NewGetTenantUsageHandler(&stubQueryTenantUsageRepository{
		usage: &ports.TenantUsageView{
			TenantID:     "tenant-a",
			StorageBytes: 2048,
			FileCount:    3,
			LastUploadAt: &lastUpload,
			UpdatedAt:    now,
		},
	})

	result, err := handler.Handle(context.Background(), queries.GetTenantUsageQuery{
		TenantID: "tenant-a",
		Auth:     mustQueryAdminContext(t, domain.RoleReadonly, "tenant-a"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.UsedStorageBytes != 2048 || result.UsedFileCount != 3 {
		t.Fatalf("unexpected usage result: %#v", result)
	}
	if result.LastUploadAt == nil || !result.LastUploadAt.Equal(lastUpload) {
		t.Fatalf("unexpected last upload time: %#v", result.LastUploadAt)
	}
}

type stubQueryTenantRepository struct {
	tenant     *domain.Tenant
	err        error
	items      []domain.Tenant
	nextCursor string
	listQuery  ports.ListTenantsQuery
}

func (s *stubQueryTenantRepository) Create(context.Context, domain.Tenant) error {
	panic("unexpected call")
}

func (s *stubQueryTenantRepository) GetByID(context.Context, string) (*domain.Tenant, error) {
	return s.tenant, s.err
}

func (s *stubQueryTenantRepository) List(_ context.Context, query ports.ListTenantsQuery) ([]domain.Tenant, string, error) {
	s.listQuery = query
	return s.items, s.nextCursor, s.err
}

func (s *stubQueryTenantRepository) Patch(context.Context, string, domain.TenantPatch) (*domain.Tenant, error) {
	panic("unexpected call")
}

type stubQueryTenantPolicyRepository struct {
	policy *ports.TenantPolicyView
	err    error
}

func (s *stubQueryTenantPolicyRepository) CreateDefault(context.Context, string) error {
	panic("unexpected call")
}

func (s *stubQueryTenantPolicyRepository) GetByTenantID(context.Context, string) (*ports.TenantPolicyView, error) {
	return s.policy, s.err
}

func (s *stubQueryTenantPolicyRepository) Patch(context.Context, string, domain.TenantPolicyPatch) (*ports.TenantPolicyView, error) {
	panic("unexpected call")
}

type stubQueryTenantUsageRepository struct {
	usage *ports.TenantUsageView
	err   error
}

func (s *stubQueryTenantUsageRepository) Initialize(context.Context, string) error {
	panic("unexpected call")
}

func (s *stubQueryTenantUsageRepository) GetByTenantID(context.Context, string) (*ports.TenantUsageView, error) {
	return s.usage, s.err
}

func mustQueryAdminContext(t *testing.T, role domain.Role, scope string) domain.AdminContext {
	t.Helper()
	return mustQueryAdminContextWithScopes(t, role, scope)
}

func mustQueryAdminContextWithScopes(t *testing.T, role domain.Role, scopes ...string) domain.AdminContext {
	t.Helper()

	ctx, err := domain.NewAdminContext(domain.AdminContextInput{
		AdminID:      "admin-1",
		Roles:        []string{string(role)},
		TenantScopes: scopes,
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
