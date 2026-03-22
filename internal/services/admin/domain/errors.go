package domain

import "github.com/architectcgz/zhi-file-service-go/pkg/xerrors"

const (
	CodeAdminPermissionDenied xerrors.Code = "ADMIN_PERMISSION_DENIED"
	CodeTenantScopeDenied     xerrors.Code = "TENANT_SCOPE_DENIED"
	CodeTenantNotFound        xerrors.Code = "TENANT_NOT_FOUND"
	CodeTenantStatusInvalid   xerrors.Code = "TENANT_STATUS_INVALID"
	CodeTenantPolicyInvalid   xerrors.Code = "TENANT_POLICY_INVALID"
	CodeAuditQueryInvalid     xerrors.Code = "AUDIT_QUERY_INVALID"
)

func ErrAdminPermissionDenied(operation Operation, minimumRole Role) error {
	return xerrors.New(CodeAdminPermissionDenied, "admin permission denied", xerrors.Details{
		"operation":    operation,
		"requiredRole": minimumRole,
	})
}

func ErrTenantScopeDenied(tenantID string) error {
	return xerrors.New(CodeTenantScopeDenied, "tenant scope denied", xerrors.Details{
		"tenantId": tenantID,
	})
}

func ErrTenantNotFound(tenantID string) error {
	return xerrors.New(CodeTenantNotFound, "tenant not found", xerrors.Details{
		"tenantId": tenantID,
	})
}

func ErrTenantStatusInvalid(tenantID string, status TenantStatus) error {
	return xerrors.New(CodeTenantStatusInvalid, "tenant status invalid", xerrors.Details{
		"tenantId": tenantID,
		"status":   status,
	})
}

func ErrTenantPolicyInvalid(field string, reason string) error {
	return xerrors.New(CodeTenantPolicyInvalid, "tenant policy invalid", xerrors.Details{
		"field":  field,
		"reason": reason,
	})
}

func ErrAuditQueryInvalid(field string, reason string) error {
	return xerrors.New(CodeAuditQueryInvalid, "audit query invalid", xerrors.Details{
		"field":  field,
		"reason": reason,
	})
}
