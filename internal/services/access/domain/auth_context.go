package domain

import (
	"slices"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const ScopeFileRead = "file:read"

type AuthContext struct {
	RequestID   string
	SubjectID   string
	SubjectType string
	TenantID    string
	ClientID    string
	TokenID     string
	Scopes      []string
}

func (a AuthContext) HasScope(scope string) bool {
	return slices.Contains(a.Scopes, scope)
}

func (a AuthContext) RequireFileRead() error {
	if a.SubjectID == "" {
		return xerrors.New(xerrors.CodeForbidden, "subject is required", nil)
	}
	if a.TenantID == "" {
		return xerrors.New(xerrors.CodeForbidden, "tenant is required", nil)
	}
	if !a.HasScope(ScopeFileRead) {
		return xerrors.New(xerrors.CodeForbidden, "file read scope is required", xerrors.Details{
			"requiredScope": ScopeFileRead,
		})
	}

	return nil
}
