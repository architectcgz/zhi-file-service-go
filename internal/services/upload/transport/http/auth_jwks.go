package httptransport

import (
	"fmt"
	"net/http"

	platformauth "github.com/architectcgz/zhi-file-service-go/internal/platform/auth/dataplane"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
)

func NewJWKSAuthResolver(source string) (AuthFunc, error) {
	return NewJWKSAuthResolverWithIssuers(source, nil)
}

func NewJWKSAuthResolverWithIssuers(source string, allowedIssuers []string) (AuthFunc, error) {
	resolver, err := platformauth.NewJWKSResolverWithIssuers(source, allowedIssuers)
	if err != nil {
		return nil, fmt.Errorf("new upload jwks resolver: %w", err)
	}

	return func(r *http.Request) (domain.AuthContext, error) {
		claims, err := resolver(r)
		if err != nil {
			return domain.AuthContext{}, err
		}
		return domain.AuthContext{
			RequestID:   requestIDFromRequest(r, ""),
			SubjectID:   claims.SubjectID,
			SubjectType: claims.SubjectType,
			TenantID:    claims.TenantID,
			ClientID:    claims.ClientID,
			TokenID:     claims.TokenID,
			Scopes:      append([]string(nil), claims.Scopes...),
		}, nil
	}, nil
}
