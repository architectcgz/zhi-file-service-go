package domain

import (
	"slices"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const GlobalTenantScope = "*"

type Role string

const (
	RoleReadonly   Role = "admin.readonly"
	RoleGovernance Role = "admin.governance"
	RoleSuper      Role = "admin.super"
)

type AdminContextInput struct {
	RequestID    string
	AdminID      string
	Roles        []string
	TenantScopes []string
	Permissions  []string
	TokenID      string
}

type AdminContext struct {
	RequestID    string
	AdminID      string
	Roles        []Role
	TenantScopes []string
	Permissions  []string
	TokenID      string
}

func NewAdminContext(input AdminContextInput) (AdminContext, error) {
	adminID := strings.TrimSpace(input.AdminID)
	if adminID == "" {
		return AdminContext{}, xerrors.New(xerrors.CodeInvalidArgument, "admin id is required", xerrors.Details{
			"field": "sub",
		})
	}

	roles, err := normalizeRoles(input.Roles)
	if err != nil {
		return AdminContext{}, err
	}

	return AdminContext{
		RequestID:    strings.TrimSpace(input.RequestID),
		AdminID:      adminID,
		Roles:        roles,
		TenantScopes: normalizeStringSet(input.TenantScopes, true),
		Permissions:  normalizeStringSet(input.Permissions, false),
		TokenID:      strings.TrimSpace(input.TokenID),
	}, nil
}

func (c AdminContext) IsGlobalScope() bool {
	return slices.Contains(c.TenantScopes, GlobalTenantScope)
}

func (c AdminContext) CanAccessTenant(tenantID string) bool {
	if c.IsGlobalScope() {
		return true
	}

	return slices.Contains(c.TenantScopes, strings.TrimSpace(tenantID))
}

func (c AdminContext) HasMinimumRole(required Role) bool {
	requiredRank, ok := roleRank(required)
	if !ok {
		return false
	}

	for _, role := range c.Roles {
		rank, ok := roleRank(role)
		if ok && rank >= requiredRank {
			return true
		}
	}

	return false
}

func normalizeRoles(values []string) ([]Role, error) {
	seen := make(map[Role]struct{}, len(values))
	roles := make([]Role, 0, len(values))
	for _, value := range values {
		role := Role(strings.TrimSpace(value))
		if role == "" {
			continue
		}
		if _, ok := roleRank(role); !ok {
			return nil, xerrors.New(xerrors.CodeInvalidArgument, "admin role is invalid", xerrors.Details{
				"field": "roles",
				"role":  role,
			})
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	if len(roles) == 0 {
		return nil, xerrors.New(xerrors.CodeInvalidArgument, "at least one admin role is required", xerrors.Details{
			"field": "roles",
		})
	}

	slices.SortFunc(roles, func(a Role, b Role) int {
		ar, _ := roleRank(a)
		br, _ := roleRank(b)
		switch {
		case ar < br:
			return -1
		case ar > br:
			return 1
		default:
			return strings.Compare(string(a), string(b))
		}
	})

	return roles, nil
}

func normalizeStringSet(values []string, defaultGlobal bool) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if normalized == GlobalTenantScope {
			return []string{GlobalTenantScope}
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	if defaultGlobal && len(result) == 0 {
		return []string{GlobalTenantScope}
	}

	return result
}

func roleRank(role Role) (int, bool) {
	switch role {
	case RoleReadonly:
		return 1, true
	case RoleGovernance:
		return 2, true
	case RoleSuper:
		return 3, true
	default:
		return 0, false
	}
}
