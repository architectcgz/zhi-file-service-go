package storageinfra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	platformstorage "github.com/architectcgz/zhi-file-service-go/internal/platform/storage"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

func TestResolveObjectURLUsesPublicBaseURL(t *testing.T) {
	adapter := &Adapter{
		cfg: config.StorageConfig{
			PublicBaseURL: "https://cdn.example.com/public",
		},
	}

	resolved, err := adapter.ResolveObjectURL(pkgstorage.ObjectRef{
		Provider:   pkgstorage.ProviderS3,
		BucketName: "public",
		ObjectKey:  "tenant-a/avatar.png",
	})
	if err != nil {
		t.Fatalf("ResolveObjectURL() error = %v", err)
	}
	if resolved != "https://cdn.example.com/public/tenant-a/avatar.png" {
		t.Fatalf("resolved = %q, want %q", resolved, "https://cdn.example.com/public/tenant-a/avatar.png")
	}
}

func TestResolveObjectURLFallsBackToEndpointAndBucket(t *testing.T) {
	adapter := &Adapter{
		cfg: config.StorageConfig{
			Endpoint: "https://storage.example.com",
		},
	}

	resolved, err := adapter.ResolveObjectURL(pkgstorage.ObjectRef{
		Provider:   pkgstorage.ProviderS3,
		BucketName: "public-bucket",
		ObjectKey:  "tenant-a/avatar.png",
	})
	if err != nil {
		t.Fatalf("ResolveObjectURL() error = %v", err)
	}
	if resolved != "https://storage.example.com/public-bucket/tenant-a/avatar.png" {
		t.Fatalf("resolved = %q, want endpoint fallback", resolved)
	}
}

func TestPresignGetObjectReturnsURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()

	adapter := newAdapterForTest(t, server.URL)
	url, err := adapter.PresignGetObject(context.Background(), pkgstorage.ObjectRef{
		Provider:   pkgstorage.ProviderS3,
		BucketName: "private-bucket",
		ObjectKey:  "tenant-a/invoice.pdf",
	}, 0)
	if err != nil {
		t.Fatalf("PresignGetObject() error = %v", err)
	}
	if !strings.Contains(url, "private-bucket/tenant-a/invoice.pdf") {
		t.Fatalf("unexpected presigned url: %s", url)
	}
}

func newAdapterForTest(t *testing.T, endpoint string) *Adapter {
	t.Helper()

	client, err := platformstorage.Open(config.StorageConfig{
		Endpoint:       endpoint,
		Region:         "us-east-1",
		AccessKey:      "key",
		SecretKey:      "secret",
		PublicBucket:   "public-bucket",
		PrivateBucket:  "private-bucket",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	adapter, err := NewAdapter(client, config.StorageConfig{
		Endpoint:       endpoint,
		Region:         "us-east-1",
		AccessKey:      "key",
		SecretKey:      "secret",
		PublicBucket:   "public-bucket",
		PrivateBucket:  "private-bucket",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}
	return adapter
}
