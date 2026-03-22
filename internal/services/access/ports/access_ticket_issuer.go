package ports

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
)

type AccessTicketIssuer interface {
	Issue(ctx context.Context, claims domain.AccessTicketClaims) (string, error)
	Verify(ctx context.Context, ticket string) (domain.AccessTicketClaims, error)
}
