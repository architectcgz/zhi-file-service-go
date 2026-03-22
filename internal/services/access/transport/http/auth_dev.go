package httptransport

import (
	"net/http"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type DevelopmentAuthConfig struct {
	Token       string
	TenantID    string
	SubjectID   string
	SubjectType string
	ClientID    string
	TokenID     string
	Scopes      []string
}

func NewDevelopmentAuthResolver(cfg DevelopmentAuthConfig) AuthFunc {
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		token = "dev-token"
	}
	tenantID := strings.TrimSpace(cfg.TenantID)
	if tenantID == "" {
		tenantID = "demo"
	}
	subjectID := strings.TrimSpace(cfg.SubjectID)
	if subjectID == "" {
		subjectID = "dev-user"
	}
	subjectType := strings.TrimSpace(cfg.SubjectType)
	if subjectType == "" {
		subjectType = "USER"
	}
	scopes := normalizeScopes(cfg.Scopes)

	return func(r *http.Request) (domain.AuthContext, error) {
		bearer := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(bearer), "bearer ") {
			return domain.AuthContext{}, xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
		}
		if strings.TrimSpace(bearer[7:]) != token {
			return domain.AuthContext{}, xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
		}

		return domain.AuthContext{
			RequestID:   requestIDFromRequest(r, ""),
			SubjectID:   subjectID,
			SubjectType: subjectType,
			TenantID:    tenantID,
			ClientID:    strings.TrimSpace(cfg.ClientID),
			TokenID:     strings.TrimSpace(cfg.TokenID),
			Scopes:      scopes,
		}, nil
	}
}

func normalizeScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{domain.ScopeFileRead}
	}

	values := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		values = append(values, scope)
	}
	if len(values) == 0 {
		return []string{domain.ScopeFileRead}
	}
	return values
}
