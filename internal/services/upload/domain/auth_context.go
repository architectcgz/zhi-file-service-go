package domain

import (
	"strings"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const ScopeFileWrite = "file:write"

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
	for _, current := range a.Scopes {
		if strings.TrimSpace(current) == scope {
			return true
		}
	}

	return false
}

func (a AuthContext) RequireFileWrite() error {
	if strings.TrimSpace(a.SubjectID) == "" {
		return xerrors.New(xerrors.CodeForbidden, "subject is required", nil)
	}
	if strings.TrimSpace(a.TenantID) == "" {
		return xerrors.New(xerrors.CodeForbidden, "tenant is required", nil)
	}
	if !a.HasScope(ScopeFileWrite) {
		return xerrors.New(xerrors.CodeForbidden, "file write scope is required", xerrors.Details{
			"requiredScope": ScopeFileWrite,
		})
	}

	return nil
}

func (a AuthContext) EnsureSessionAccess(session *Session) error {
	if session == nil {
		return xerrors.New(xerrors.CodeInternalError, "upload session is required", nil)
	}
	if strings.TrimSpace(a.TenantID) != session.TenantID {
		return xerrors.New(xerrors.CodeForbidden, "tenant does not match upload session", xerrors.Details{
			"resourceType": "uploadSession",
			"resourceId":   session.ID,
		})
	}
	if strings.TrimSpace(a.SubjectID) != session.OwnerID {
		return xerrors.New(xerrors.CodeForbidden, "subject does not own upload session", xerrors.Details{
			"resourceType": "uploadSession",
			"resourceId":   session.ID,
		})
	}

	return nil
}
