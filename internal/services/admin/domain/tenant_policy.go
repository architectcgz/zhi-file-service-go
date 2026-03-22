package domain

import (
	"slices"
	"strings"

	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type TenantPolicy struct {
	MaxStorageBytes    *int64
	MaxFileCount       *int64
	MaxSingleFileSize  *int64
	AllowedMimeTypes   []string
	AllowedExtensions  []string
	DefaultAccessLevel *string
	AutoCreateEnabled  *bool
}

type TenantPolicyPatch struct {
	MaxStorageBytes    *int64
	MaxFileCount       *int64
	MaxSingleFileSize  *int64
	AllowedMimeTypes   []string
	AllowedExtensions  []string
	DefaultAccessLevel *string
	AutoCreateEnabled  *bool
	Reason             string
}

func (p TenantPolicy) Normalize() TenantPolicy {
	return TenantPolicy{
		MaxStorageBytes:    p.MaxStorageBytes,
		MaxFileCount:       p.MaxFileCount,
		MaxSingleFileSize:  p.MaxSingleFileSize,
		AllowedMimeTypes:   normalizeStringSlice(p.AllowedMimeTypes, false),
		AllowedExtensions:  normalizeStringSlice(p.AllowedExtensions, true),
		DefaultAccessLevel: normalizeAccessLevelPtr(p.DefaultAccessLevel),
		AutoCreateEnabled:  p.AutoCreateEnabled,
	}
}

func (p TenantPolicy) Validate() error {
	if err := validatePositiveInt64Ptr("maxStorageBytes", p.MaxStorageBytes); err != nil {
		return err
	}
	if err := validatePositiveInt64Ptr("maxFileCount", p.MaxFileCount); err != nil {
		return err
	}
	if err := validatePositiveInt64Ptr("maxSingleFileSize", p.MaxSingleFileSize); err != nil {
		return err
	}
	if err := validateAccessLevelPtr(p.DefaultAccessLevel); err != nil {
		return err
	}

	return nil
}

func (p TenantPolicyPatch) Normalize() TenantPolicyPatch {
	return TenantPolicyPatch{
		MaxStorageBytes:    p.MaxStorageBytes,
		MaxFileCount:       p.MaxFileCount,
		MaxSingleFileSize:  p.MaxSingleFileSize,
		AllowedMimeTypes:   normalizeStringSlice(p.AllowedMimeTypes, false),
		AllowedExtensions:  normalizeStringSlice(p.AllowedExtensions, true),
		DefaultAccessLevel: normalizeAccessLevelPtr(p.DefaultAccessLevel),
		AutoCreateEnabled:  p.AutoCreateEnabled,
		Reason:             strings.TrimSpace(p.Reason),
	}
}

func (p TenantPolicyPatch) Validate() error {
	return p.ApplyTo(TenantPolicy{}).Validate()
}

func (p TenantPolicyPatch) IsEmpty() bool {
	return p.MaxStorageBytes == nil &&
		p.MaxFileCount == nil &&
		p.MaxSingleFileSize == nil &&
		p.AllowedMimeTypes == nil &&
		p.AllowedExtensions == nil &&
		p.DefaultAccessLevel == nil &&
		p.AutoCreateEnabled == nil
}

func (p TenantPolicy) TightensComparedTo(current TenantPolicy) bool {
	if int64LimitTightens(current.MaxStorageBytes, p.MaxStorageBytes) {
		return true
	}
	if int64LimitTightens(current.MaxFileCount, p.MaxFileCount) {
		return true
	}
	if int64LimitTightens(current.MaxSingleFileSize, p.MaxSingleFileSize) {
		return true
	}
	if stringSetTightens(current.AllowedMimeTypes, p.AllowedMimeTypes) {
		return true
	}
	if stringSetTightens(current.AllowedExtensions, p.AllowedExtensions) {
		return true
	}
	if boolTightens(current.AutoCreateEnabled, p.AutoCreateEnabled) {
		return true
	}

	return false
}

func (p TenantPolicyPatch) ApplyTo(current TenantPolicy) TenantPolicy {
	next := current
	if p.MaxStorageBytes != nil {
		next.MaxStorageBytes = p.MaxStorageBytes
	}
	if p.MaxFileCount != nil {
		next.MaxFileCount = p.MaxFileCount
	}
	if p.MaxSingleFileSize != nil {
		next.MaxSingleFileSize = p.MaxSingleFileSize
	}
	if p.AllowedMimeTypes != nil {
		next.AllowedMimeTypes = slices.Clone(p.AllowedMimeTypes)
	}
	if p.AllowedExtensions != nil {
		next.AllowedExtensions = slices.Clone(p.AllowedExtensions)
	}
	if p.DefaultAccessLevel != nil {
		next.DefaultAccessLevel = p.DefaultAccessLevel
	}
	if p.AutoCreateEnabled != nil {
		next.AutoCreateEnabled = p.AutoCreateEnabled
	}

	return next
}

func int64LimitTightens(current *int64, next *int64) bool {
	switch {
	case current == nil && next == nil:
		return false
	case current == nil && next != nil:
		return true
	case current != nil && next == nil:
		return false
	default:
		return *next < *current
	}
}

func stringSetTightens(current []string, next []string) bool {
	if len(next) == 0 {
		return false
	}
	if len(current) == 0 {
		return true
	}
	for _, value := range next {
		if !slices.Contains(current, value) {
			return false
		}
	}
	return len(next) < len(current)
}

func boolTightens(current *bool, next *bool) bool {
	return current != nil && next != nil && *current && !*next
}

func validatePositiveInt64Ptr(field string, value *int64) error {
	if value == nil || *value > 0 {
		return nil
	}

	return ErrTenantPolicyInvalid(field, "must be greater than zero")
}

func validateAccessLevelPtr(value *string) error {
	if value == nil {
		return nil
	}

	switch pkgstorage.AccessLevel(*value) {
	case pkgstorage.AccessLevelPrivate, pkgstorage.AccessLevelPublic:
		return nil
	default:
		return ErrTenantPolicyInvalid("defaultAccessLevel", "must be PUBLIC or PRIVATE")
	}
}

func normalizeAccessLevelPtr(value *string) *string {
	if value == nil {
		return nil
	}

	normalized := strings.ToUpper(strings.TrimSpace(*value))
	return &normalized
}

func normalizeStringSlice(values []string, lower bool) []string {
	if values == nil {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if lower {
			normalized = strings.ToLower(normalized)
		}
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}
