package view

import (
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
)

type Tenant struct {
	TenantID     string
	TenantName   string
	Status       domain.TenantStatus
	ContactEmail string
	Description  string
	CreatedAt    Time
	UpdatedAt    Time
}

type TenantList struct {
	Items      []Tenant
	NextCursor string
}

func FromTenant(value domain.Tenant) Tenant {
	return Tenant{
		TenantID:     value.TenantID,
		TenantName:   value.TenantName,
		Status:       value.Status,
		ContactEmail: value.ContactEmail,
		Description:  value.Description,
		CreatedAt:    Time(value.CreatedAt),
		UpdatedAt:    Time(value.UpdatedAt),
	}
}

func FromTenants(values []domain.Tenant) []Tenant {
	items := make([]Tenant, 0, len(values))
	for _, value := range values {
		items = append(items, FromTenant(value))
	}

	return items
}
