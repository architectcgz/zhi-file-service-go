package domain

import (
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "ACTIVE"
	TenantStatusSuspended TenantStatus = "SUSPENDED"
	TenantStatusDeleted   TenantStatus = "DELETED"
)

type Tenant struct {
	TenantID     string
	TenantName   string
	Status       TenantStatus
	ContactEmail string
	Description  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type TenantPatch struct {
	TenantName   *string
	Status       *TenantStatus
	ContactEmail *string
	Description  *string
	Reason       string
}

func (s TenantStatus) IsDestructive() bool {
	switch s {
	case TenantStatusSuspended, TenantStatusDeleted:
		return true
	default:
		return false
	}
}

func TenantStatusPtr(status TenantStatus) *TenantStatus {
	return &status
}

func (s TenantStatus) Validate() error {
	switch s {
	case TenantStatusActive, TenantStatusSuspended, TenantStatusDeleted:
		return nil
	default:
		return xerrors.New(CodeTenantStatusInvalid, "tenant status invalid", xerrors.Details{
			"status": s,
		})
	}
}

func (p TenantPatch) HasDestructiveChange() bool {
	return p.Status != nil && p.Status.IsDestructive()
}

func (p TenantPatch) Normalize() TenantPatch {
	return TenantPatch{
		TenantName:   normalizeStringPtr(p.TenantName),
		Status:       p.Status,
		ContactEmail: normalizeStringPtr(p.ContactEmail),
		Description:  normalizeStringPtr(p.Description),
		Reason:       strings.TrimSpace(p.Reason),
	}
}

func (p TenantPatch) IsEmpty() bool {
	return p.TenantName == nil && p.Status == nil && p.ContactEmail == nil && p.Description == nil
}

func normalizeStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	return &normalized
}
