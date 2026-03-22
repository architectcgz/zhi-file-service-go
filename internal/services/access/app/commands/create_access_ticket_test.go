package commands_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	accessidempotency "github.com/architectcgz/zhi-file-service-go/internal/services/access/infra/idempotency"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestCreateAccessTicketBindsTenantAndSubject(t *testing.T) {
	now := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
	repo := &stubFileReadRepository{
		file: domain.FileView{
			FileID:          "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
			TenantID:        "tenant-a",
			FileName:        "invoice.pdf",
			AccessLevel:     storage.AccessLevelPrivate,
			Status:          domain.FileStatusActive,
			StorageProvider: storage.ProviderS3,
			BucketName:      "private",
			ObjectKey:       "tenant-a/invoice.pdf",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	issuer := &stubTicketIssuer{ticket: "at_01JQ2QFJ7X0C24C25J24E2RYN9"}
	policies := &stubTenantPolicyReader{policy: domain.TenantPolicy{TenantID: "tenant-a"}}
	handler := commands.NewCreateAccessTicketHandler(repo, policies, issuer, nil, clock.NewFixed(now), 5*time.Minute, "/api/v1/access-tickets")

	result, err := handler.Handle(context.Background(), commands.CreateAccessTicketCommand{
		FileID:       repo.file.FileID,
		ExpiresIn:    2 * time.Minute,
		Disposition:  domain.DownloadDispositionAttachment,
		ResponseName: "invoice.pdf",
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  "tenant-a",
			Scopes:    []string{domain.ScopeFileRead},
		},
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.Ticket != issuer.ticket {
		t.Fatalf("expected ticket %q, got %q", issuer.ticket, result.Ticket)
	}
	if result.RedirectURL != "/api/v1/access-tickets/"+issuer.ticket+"/redirect" {
		t.Fatalf("unexpected redirect url: %s", result.RedirectURL)
	}
	if issuer.claims.TenantID != "tenant-a" {
		t.Fatalf("expected tenant claim tenant-a, got %s", issuer.claims.TenantID)
	}
	if issuer.claims.Subject != "user-1" {
		t.Fatalf("expected subject claim user-1, got %s", issuer.claims.Subject)
	}
	if issuer.claims.ExpiresAt != now.Add(2*time.Minute) {
		t.Fatalf("unexpected expires at: %s", issuer.claims.ExpiresAt)
	}
}

func TestCreateAccessTicketRejectsDownloadDisabledPolicy(t *testing.T) {
	now := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
	repo := &stubFileReadRepository{
		file: domain.FileView{
			FileID:          "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
			TenantID:        "tenant-a",
			FileName:        "invoice.pdf",
			AccessLevel:     storage.AccessLevelPrivate,
			Status:          domain.FileStatusActive,
			StorageProvider: storage.ProviderS3,
			BucketName:      "private",
			ObjectKey:       "tenant-a/invoice.pdf",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	policies := &stubTenantPolicyReader{policy: domain.TenantPolicy{
		TenantID:         "tenant-a",
		DownloadDisabled: true,
	}}
	handler := commands.NewCreateAccessTicketHandler(repo, policies, &stubTicketIssuer{}, nil, clock.NewFixed(now), 5*time.Minute, "/api/v1/access-tickets")

	_, err := handler.Handle(context.Background(), commands.CreateAccessTicketCommand{
		FileID:      repo.file.FileID,
		Disposition: domain.DownloadDispositionAttachment,
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  "tenant-a",
			Scopes:    []string{domain.ScopeFileRead},
		},
	})
	if code := xerrors.CodeOf(err); code != domain.CodeDownloadNotAllowed {
		t.Fatalf("expected download not allowed, got %s (err=%v)", code, err)
	}
}

func TestCreateAccessTicketReturnsStoredResultForSameIdempotencyKey(t *testing.T) {
	now := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
	repo := &stubFileReadRepository{
		file: domain.FileView{
			FileID:          "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
			TenantID:        "tenant-a",
			FileName:        "invoice.pdf",
			AccessLevel:     storage.AccessLevelPrivate,
			Status:          domain.FileStatusActive,
			StorageProvider: storage.ProviderS3,
			BucketName:      "private",
			ObjectKey:       "tenant-a/invoice.pdf",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	issuer := &stubTicketIssuer{
		ticket:  "at_first",
		tickets: []string{"at_first", "at_second"},
	}
	policies := &stubTenantPolicyReader{policy: domain.TenantPolicy{TenantID: "tenant-a"}}
	store := accessidempotency.NewMemoryStore(clock.NewFixed(now))
	handler := commands.NewCreateAccessTicketHandler(repo, policies, issuer, store, clock.NewFixed(now), 5*time.Minute, "/api/v1/access-tickets")
	command := commands.CreateAccessTicketCommand{
		FileID:         repo.file.FileID,
		IdempotencyKey: "ticket-001",
		ExpiresIn:      2 * time.Minute,
		Disposition:    domain.DownloadDispositionAttachment,
		ResponseName:   "invoice.pdf",
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  "tenant-a",
			Scopes:    []string{domain.ScopeFileRead},
		},
	}

	first, err := handler.Handle(context.Background(), command)
	if err != nil {
		t.Fatalf("first Handle returned error: %v", err)
	}
	second, err := handler.Handle(context.Background(), command)
	if err != nil {
		t.Fatalf("second Handle returned error: %v", err)
	}
	if first.Ticket != "at_first" {
		t.Fatalf("first ticket = %q, want %q", first.Ticket, "at_first")
	}
	if second.Ticket != first.Ticket {
		t.Fatalf("second ticket = %q, want %q", second.Ticket, first.Ticket)
	}
	if issuer.issueCalls != 1 {
		t.Fatalf("issue calls = %d, want %d", issuer.issueCalls, 1)
	}
}

func TestCreateAccessTicketRejectsIdempotencyConflict(t *testing.T) {
	now := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
	repo := &stubFileReadRepository{
		file: domain.FileView{
			FileID:          "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
			TenantID:        "tenant-a",
			FileName:        "invoice.pdf",
			AccessLevel:     storage.AccessLevelPrivate,
			Status:          domain.FileStatusActive,
			StorageProvider: storage.ProviderS3,
			BucketName:      "private",
			ObjectKey:       "tenant-a/invoice.pdf",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	policies := &stubTenantPolicyReader{policy: domain.TenantPolicy{TenantID: "tenant-a"}}
	store := accessidempotency.NewMemoryStore(clock.NewFixed(now))
	handler := commands.NewCreateAccessTicketHandler(repo, policies, &stubTicketIssuer{ticket: "at_first"}, store, clock.NewFixed(now), 5*time.Minute, "/api/v1/access-tickets")
	base := commands.CreateAccessTicketCommand{
		FileID:         repo.file.FileID,
		IdempotencyKey: "ticket-001",
		Disposition:    domain.DownloadDispositionAttachment,
		ResponseName:   "invoice.pdf",
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  "tenant-a",
			Scopes:    []string{domain.ScopeFileRead},
		},
	}

	if _, err := handler.Handle(context.Background(), base); err != nil {
		t.Fatalf("first Handle returned error: %v", err)
	}
	_, err := handler.Handle(context.Background(), commands.CreateAccessTicketCommand{
		FileID:         base.FileID,
		IdempotencyKey: base.IdempotencyKey,
		Disposition:    domain.DownloadDispositionInline,
		ResponseName:   base.ResponseName,
		Auth:           base.Auth,
	})
	if code := xerrors.CodeOf(err); code != xerrors.CodeConflict {
		t.Fatalf("expected conflict, got %s (err=%v)", code, err)
	}
}

func TestCreateAccessTicketReturnsStoredResultWithoutRecheckingRepository(t *testing.T) {
	now := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
	repo := &stubFileReadRepository{
		file: domain.FileView{
			FileID:          "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
			TenantID:        "tenant-a",
			FileName:        "invoice.pdf",
			AccessLevel:     storage.AccessLevelPrivate,
			Status:          domain.FileStatusActive,
			StorageProvider: storage.ProviderS3,
			BucketName:      "private",
			ObjectKey:       "tenant-a/invoice.pdf",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	issuer := &stubTicketIssuer{ticket: "at_first"}
	policies := &stubTenantPolicyReader{policy: domain.TenantPolicy{TenantID: "tenant-a"}}
	store := accessidempotency.NewMemoryStore(clock.NewFixed(now))
	handler := commands.NewCreateAccessTicketHandler(repo, policies, issuer, store, clock.NewFixed(now), 5*time.Minute, "/api/v1/access-tickets")
	command := commands.CreateAccessTicketCommand{
		FileID:         repo.file.FileID,
		IdempotencyKey: "ticket-001",
		Disposition:    domain.DownloadDispositionAttachment,
		ResponseName:   "invoice.pdf",
		Auth: domain.AuthContext{
			SubjectID: "user-1",
			TenantID:  "tenant-a",
			Scopes:    []string{domain.ScopeFileRead},
		},
	}

	first, err := handler.Handle(context.Background(), command)
	if err != nil {
		t.Fatalf("first Handle returned error: %v", err)
	}

	repo.err = fmt.Errorf("repository should not be called again")
	second, err := handler.Handle(context.Background(), command)
	if err != nil {
		t.Fatalf("second Handle returned error: %v", err)
	}
	if second.Ticket != first.Ticket {
		t.Fatalf("second ticket = %q, want %q", second.Ticket, first.Ticket)
	}
	if issuer.issueCalls != 1 {
		t.Fatalf("issue calls = %d, want %d", issuer.issueCalls, 1)
	}
}

type stubFileReadRepository struct {
	file domain.FileView
	err  error
}

func (s *stubFileReadRepository) GetByID(_ context.Context, _ string) (domain.FileView, error) {
	return s.file, s.err
}

type stubTicketIssuer struct {
	ticket     string
	tickets    []string
	issueCalls int
	claims     domain.AccessTicketClaims
}

func (s *stubTicketIssuer) Issue(_ context.Context, claims domain.AccessTicketClaims) (string, error) {
	s.issueCalls++
	s.claims = claims
	if len(s.tickets) >= s.issueCalls {
		return s.tickets[s.issueCalls-1], nil
	}
	if s.ticket == "" {
		return "", fmt.Errorf("ticket is not configured")
	}
	return s.ticket, nil
}

func (s *stubTicketIssuer) Verify(context.Context, string) (domain.AccessTicketClaims, error) {
	panic("unexpected call")
}

type stubTenantPolicyReader struct {
	policy domain.TenantPolicy
	err    error
}

func (s *stubTenantPolicyReader) GetByTenantID(context.Context, string) (domain.TenantPolicy, error) {
	return s.policy, s.err
}
