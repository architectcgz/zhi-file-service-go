package view

import "github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"

type AuditLog struct {
	AuditLogID string
	TenantID   string
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	Details    map[string]any
	CreatedAt  Time
}

type AuditLogList struct {
	Items      []AuditLog
	NextCursor string
}

func FromAuditLog(value ports.AuditLogRecord) AuditLog {
	return AuditLog{
		AuditLogID: value.AuditID,
		TenantID:   value.TenantID,
		ActorID:    value.AdminSubject,
		Action:     value.Action,
		TargetType: value.TargetType,
		TargetID:   value.TargetID,
		Details:    cloneMap(value.Details),
		CreatedAt:  Time(value.CreatedAt),
	}
}

func FromAuditLogs(values []ports.AuditLogRecord) []AuditLog {
	items := make([]AuditLog, 0, len(values))
	for _, value := range values {
		items = append(items, FromAuditLog(value))
	}

	return items
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}

	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = item
	}

	return cloned
}
