package domain

import "slices"

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
