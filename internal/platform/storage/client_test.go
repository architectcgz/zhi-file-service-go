package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
)

func TestValidateHeadsConfiguredBuckets(t *testing.T) {
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		requests[r.URL.Path]++
		switch r.URL.Path {
		case "/public", "/private":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := Open(config.StorageConfig{
		Endpoint:       server.URL,
		Region:         "us-east-1",
		AccessKey:      "key",
		SecretKey:      "secret",
		PublicBucket:   "public",
		PrivateBucket:  "private",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if requests["/public"] != 1 || requests["/private"] != 1 {
		t.Fatalf("unexpected request counts: %#v", requests)
	}
}

func TestValidateReturnsErrorWhenBucketIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/public":
			w.WriteHeader(http.StatusOK)
		case "/private":
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := Open(config.StorageConfig{
		Endpoint:       server.URL,
		Region:         "us-east-1",
		AccessKey:      "key",
		SecretKey:      "secret",
		PublicBucket:   "public",
		PrivateBucket:  "private",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if err := client.Validate(context.Background()); err == nil {
		t.Fatal("expected validation error")
	}
}
