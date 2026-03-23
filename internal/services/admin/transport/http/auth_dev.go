package httptransport

import (
	"net/http"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type DevelopmentAuthConfig struct {
	Token        string
	AdminID      string
	Roles        []string
	TenantScopes []string
	Permissions  []string
	TokenID      string
}

func NewDevelopmentAuthResolver(cfg DevelopmentAuthConfig) AuthFunc {
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		token = "dev-token"
	}
	adminID := strings.TrimSpace(cfg.AdminID)
	if adminID == "" {
		adminID = "admin-dev"
	}
	roles := normalizeStrings(cfg.Roles, []string{string(domain.RoleSuper)})
	tenantScopes := normalizeStrings(cfg.TenantScopes, []string{domain.GlobalTenantScope})
	permissions := normalizeStrings(cfg.Permissions, nil)
	tokenID := strings.TrimSpace(cfg.TokenID)

	return func(r *http.Request) (domain.AdminContext, error) {
		bearer := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(bearer), "bearer ") {
			return domain.AdminContext{}, xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
		}
		if strings.TrimSpace(bearer[7:]) != token {
			return domain.AdminContext{}, xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
		}

		auth, err := domain.NewAdminContext(domain.AdminContextInput{
			RequestID:    requestIDFromRequest(r, ""),
			AdminID:      adminID,
			Roles:        roles,
			TenantScopes: tenantScopes,
			Permissions:  permissions,
			TokenID:      tokenID,
		})
		if err != nil {
			return domain.AdminContext{}, err
		}
		return auth, nil
	}
}

func normalizeStrings(values []string, defaults []string) []string {
	if len(values) == 0 {
		return cloneStrings(defaults)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return cloneStrings(defaults)
	}
	return result
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, value)
	}
	return cloned
}
