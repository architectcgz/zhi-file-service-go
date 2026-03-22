package storage

import "testing"

func TestBucketRefValidate(t *testing.T) {
	valid := BucketRef{
		Provider:   ProviderMinIO,
		BucketName: "zhi-files-public",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid bucket ref, got %v", err)
	}

	invalid := BucketRef{BucketName: "zhi-files-public"}
	if err := invalid.Validate(); err == nil {
		t.Fatalf("expected error for missing provider")
	}
}

func TestObjectRefValidate(t *testing.T) {
	valid := ObjectRef{
		Provider:   ProviderS3,
		BucketName: "zhi-files-private",
		ObjectKey:  "demo/file.txt",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid object ref, got %v", err)
	}

	invalid := ObjectRef{
		Provider:   ProviderS3,
		BucketName: "zhi-files-private",
	}
	if err := invalid.Validate(); err == nil {
		t.Fatalf("expected error for missing object key")
	}
}
