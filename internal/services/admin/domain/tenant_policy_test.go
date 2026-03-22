package domain

import "testing"

func TestTenantStatus_IsDestructive(t *testing.T) {
	t.Parallel()

	if TenantStatusActive.IsDestructive() {
		t.Fatal("ACTIVE IsDestructive() = true, want false")
	}
	if !TenantStatusSuspended.IsDestructive() {
		t.Fatal("SUSPENDED IsDestructive() = false, want true")
	}
	if !TenantStatusDeleted.IsDestructive() {
		t.Fatal("DELETED IsDestructive() = false, want true")
	}
}

func TestTenantPolicy_TightensComparedTo(t *testing.T) {
	t.Parallel()

	current := TenantPolicy{
		MaxStorageBytes:   int64Ptr(100),
		MaxFileCount:      int64Ptr(10),
		MaxSingleFileSize: int64Ptr(50),
		AllowedMimeTypes:  []string{"image/png", "image/jpeg"},
		AllowedExtensions: []string{".png", ".jpg"},
		DefaultAccessLevel: stringPtr("PRIVATE"),
		AutoCreateEnabled: boolPtr(true),
	}

	tighter := TenantPolicy{
		MaxStorageBytes:   int64Ptr(80),
		MaxFileCount:      int64Ptr(8),
		MaxSingleFileSize: int64Ptr(40),
		AllowedMimeTypes:  []string{"image/png"},
		AllowedExtensions: []string{".png"},
		DefaultAccessLevel: stringPtr("PUBLIC"),
		AutoCreateEnabled: boolPtr(false),
	}

	if !tighter.TightensComparedTo(current) {
		t.Fatal("TightensComparedTo() = false, want true")
	}

	looser := TenantPolicy{
		MaxStorageBytes:   int64Ptr(120),
		MaxFileCount:      int64Ptr(20),
		MaxSingleFileSize: int64Ptr(60),
		AllowedMimeTypes:  []string{"image/png", "image/jpeg", "application/pdf"},
		AllowedExtensions: []string{".png", ".jpg", ".pdf"},
		AutoCreateEnabled: boolPtr(true),
	}

	if looser.TightensComparedTo(current) {
		t.Fatal("looser TightensComparedTo() = true, want false")
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
