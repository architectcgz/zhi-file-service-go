package queries_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/app/queries"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestRedirectByAccessTicketResolvesPrivateDownload(t *testing.T) {
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
	issuer := &stubRedirectTicketIssuer{
		claims: domain.AccessTicketClaims{
			FileID:      repo.file.FileID,
			TenantID:    "tenant-a",
			Subject:     "user-1",
			SubjectType: "USER",
			Disposition: domain.DownloadDispositionAttachment,
			ExpiresAt:   now.Add(2 * time.Minute),
		},
	}
	presign := &stubPresignManager{url: "https://s3.example.com/private/object?signature=1"}
	handler := queries.NewRedirectByAccessTicketHandler(repo, policies, issuer, &stubObjectLocator{}, presign, clock.NewFixed(now), 2*time.Minute, true)

	result, err := handler.Handle(context.Background(), queries.RedirectByAccessTicketQuery{
		Ticket: "at_01JQ2QFJ7X0C24C25J24E2RYN9",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.URL != presign.url {
		t.Fatalf("expected url %q, got %q", presign.url, result.URL)
	}
	if issuer.verifyCalls != 1 {
		t.Fatalf("expected verify calls 1, got %d", issuer.verifyCalls)
	}
}

func TestRedirectByAccessTicketRejectsExpiredTicket(t *testing.T) {
	now := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
	handler := queries.NewRedirectByAccessTicketHandler(
		&stubFileReadRepository{},
		&stubTenantPolicyReader{},
		&stubRedirectTicketIssuer{
			claims: domain.AccessTicketClaims{
				FileID:      "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
				TenantID:    "tenant-a",
				Subject:     "user-1",
				SubjectType: "USER",
				Disposition: domain.DownloadDispositionAttachment,
				ExpiresAt:   now.Add(-time.Second),
			},
		},
		&stubObjectLocator{},
		&stubPresignManager{},
		clock.NewFixed(now),
		2*time.Minute,
		true,
	)

	_, err := handler.Handle(context.Background(), queries.RedirectByAccessTicketQuery{Ticket: "expired-ticket"})
	if code := xerrors.CodeOf(err); code != domain.CodeAccessTicketExpired {
		t.Fatalf("expected access ticket expired, got %s (err=%v)", code, err)
	}
}

type stubRedirectTicketIssuer struct {
	claims      domain.AccessTicketClaims
	verifyErr   error
	verifyCalls int
}

func (s *stubRedirectTicketIssuer) Issue(context.Context, domain.AccessTicketClaims) (string, error) {
	panic("unexpected call")
}

func (s *stubRedirectTicketIssuer) Verify(context.Context, string) (domain.AccessTicketClaims, error) {
	s.verifyCalls++
	if s.verifyErr != nil {
		return domain.AccessTicketClaims{}, s.verifyErr
	}
	return s.claims, nil
}

func TestRedirectByAccessTicketRejectsInvalidTicket(t *testing.T) {
	handler := queries.NewRedirectByAccessTicketHandler(
		&stubFileReadRepository{},
		&stubTenantPolicyReader{},
		&stubRedirectTicketIssuer{verifyErr: errors.New("invalid signature")},
		&stubObjectLocator{},
		&stubPresignManager{},
		clock.NewFixed(time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)),
		2*time.Minute,
		true,
	)

	_, err := handler.Handle(context.Background(), queries.RedirectByAccessTicketQuery{Ticket: "broken-ticket"})
	if code := xerrors.CodeOf(err); code != domain.CodeAccessTicketInvalid {
		t.Fatalf("expected access ticket invalid, got %s (err=%v)", code, err)
	}
}
