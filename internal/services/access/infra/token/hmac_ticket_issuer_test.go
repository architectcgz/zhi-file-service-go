package token

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
)

func TestHMACTicketIssuerIssueAndVerifyRoundTrip(t *testing.T) {
	issuer, err := NewHMACTicketIssuer("secret-key")
	if err != nil {
		t.Fatalf("NewHMACTicketIssuer() error = %v", err)
	}

	claims := domain.AccessTicketClaims{
		FileID:       "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
		TenantID:     "tenant-a",
		Subject:      "user-1",
		SubjectType:  "USER",
		Disposition:  domain.DownloadDispositionAttachment,
		ResponseName: "invoice.pdf",
		ExpiresAt:    time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
	}

	ticket, err := issuer.Issue(context.Background(), claims)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if !strings.HasPrefix(ticket, ticketPrefix) {
		t.Fatalf("ticket = %q, want prefix %q", ticket, ticketPrefix)
	}

	verified, err := issuer.Verify(context.Background(), ticket)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verified.FileID != claims.FileID || verified.TenantID != claims.TenantID {
		t.Fatalf("verified claims mismatch: %#v", verified)
	}
	if !verified.ExpiresAt.Equal(claims.ExpiresAt) {
		t.Fatalf("expiresAt = %s, want %s", verified.ExpiresAt, claims.ExpiresAt)
	}
}

func TestHMACTicketIssuerRejectsTamperedTicket(t *testing.T) {
	issuer, err := NewHMACTicketIssuer("secret-key")
	if err != nil {
		t.Fatalf("NewHMACTicketIssuer() error = %v", err)
	}

	ticket, err := issuer.Issue(context.Background(), domain.AccessTicketClaims{
		FileID:      "01JQ2QFJ1KRYT0X8S6Q9S7D9A1",
		TenantID:    "tenant-a",
		Subject:     "user-1",
		Disposition: domain.DownloadDispositionAttachment,
		ExpiresAt:   time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	_, err = issuer.Verify(context.Background(), ticket+"broken")
	if err == nil {
		t.Fatal("expected Verify() error")
	}
}
