package domain

import (
	"strings"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type Operation string

const (
	OperationListTenants      Operation = "listTenants"
	OperationCreateTenant     Operation = "createTenant"
	OperationGetTenant        Operation = "getTenant"
	OperationPatchTenant      Operation = "patchTenant"
	OperationGetTenantPolicy  Operation = "getTenantPolicy"
	OperationPatchTenantPolicy Operation = "patchTenantPolicy"
	OperationGetTenantUsage   Operation = "getTenantUsage"
	OperationListFiles        Operation = "listFiles"
	OperationGetFile          Operation = "getFile"
	OperationDeleteFile       Operation = "deleteFile"
	OperationListAuditLogs    Operation = "listAuditLogs"
)

type OperationRule struct {
	MinimumRole Role
	Destructive bool
}

var operationRules = map[Operation]OperationRule{
	OperationListTenants:       {MinimumRole: RoleReadonly},
	OperationCreateTenant:      {MinimumRole: RoleSuper},
	OperationGetTenant:         {MinimumRole: RoleReadonly},
	OperationPatchTenant:       {MinimumRole: RoleGovernance},
	OperationGetTenantPolicy:   {MinimumRole: RoleReadonly},
	OperationPatchTenantPolicy: {MinimumRole: RoleGovernance},
	OperationGetTenantUsage:    {MinimumRole: RoleReadonly},
	OperationListFiles:         {MinimumRole: RoleReadonly},
	OperationGetFile:           {MinimumRole: RoleReadonly},
	OperationDeleteFile:        {MinimumRole: RoleGovernance, Destructive: true},
	OperationListAuditLogs:     {MinimumRole: RoleReadonly},
}

func RuleFor(operation Operation) (OperationRule, bool) {
	rule, ok := operationRules[operation]
	return rule, ok
}

func AuthorizeOperation(ctx AdminContext, operation Operation) error {
	rule, ok := RuleFor(operation)
	if !ok {
		return xerrors.New(xerrors.CodeInternalError, "admin operation is not registered", xerrors.Details{
			"operation": operation,
		})
	}
	if ctx.HasMinimumRole(rule.MinimumRole) {
		return nil
	}

	return ErrAdminPermissionDenied(operation, rule.MinimumRole)
}

func EnsureTenantScope(ctx AdminContext, tenantID string) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return xerrors.New(xerrors.CodeInvalidArgument, "tenant id is required", xerrors.Details{
			"field": "tenantId",
		})
	}
	if ctx.CanAccessTenant(tenantID) {
		return nil
	}

	return ErrTenantScopeDenied(tenantID)
}

func RequireDestructiveReason(reason string) error {
	if strings.TrimSpace(reason) != "" {
		return nil
	}

	return xerrors.New(xerrors.CodeInvalidArgument, "reason is required for destructive operation", xerrors.Details{
		"field": "reason",
	})
}
