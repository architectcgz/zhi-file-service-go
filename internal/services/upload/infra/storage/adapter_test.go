package storageinfra

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	platformstorage "github.com/architectcgz/zhi-file-service-go/internal/platform/storage"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

func TestResolveUsesConfiguredBuckets(t *testing.T) {
	adapter := &Adapter{
		cfg: config.StorageConfig{
			PublicBucket:  "public-bucket",
			PrivateBucket: "private-bucket",
			PublicBaseURL: "https://cdn.example.com",
		},
		provider: pkgstorage.ProviderS3,
	}

	publicBucket, err := adapter.Resolve(pkgstorage.AccessLevelPublic)
	if err != nil {
		t.Fatalf("Resolve(public) error = %v", err)
	}
	if publicBucket.BucketName != "public-bucket" {
		t.Fatalf("public bucket = %q, want %q", publicBucket.BucketName, "public-bucket")
	}
	if publicBucket.PublicBase != "https://cdn.example.com" {
		t.Fatalf("public base = %q, want %q", publicBucket.PublicBase, "https://cdn.example.com")
	}

	privateBucket, err := adapter.Resolve(pkgstorage.AccessLevelPrivate)
	if err != nil {
		t.Fatalf("Resolve(private) error = %v", err)
	}
	if privateBucket.BucketName != "private-bucket" {
		t.Fatalf("private bucket = %q, want %q", privateBucket.BucketName, "private-bucket")
	}
}

func TestHeadObjectDecodesSHA256Checksum(t *testing.T) {
	const checksumHex = "4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		switch r.URL.Path {
		case "/public-bucket":
			w.WriteHeader(http.StatusOK)
		case "/private-bucket":
			w.WriteHeader(http.StatusOK)
		case "/private-bucket/tenant-a/uploads/object.txt":
			w.Header().Set("Content-Length", "5")
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("ETag", `"etag-1"`)
			w.Header().Set("x-amz-checksum-sha256", base64.StdEncoding.EncodeToString(mustHex(t, checksumHex)))
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := newAdapterForTest(t, server.URL)
	metadata, err := adapter.HeadObject(context.Background(), pkgstorage.ObjectRef{
		Provider:   pkgstorage.ProviderS3,
		BucketName: "private-bucket",
		ObjectKey:  "tenant-a/uploads/object.txt",
	})
	if err != nil {
		t.Fatalf("HeadObject error = %v", err)
	}
	if metadata.Checksum != checksumHex {
		t.Fatalf("checksum = %q, want %q", metadata.Checksum, checksumHex)
	}
	if metadata.ETag != `"etag-1"` {
		t.Fatalf("etag = %q, want %q", metadata.ETag, `"etag-1"`)
	}
}

func TestComputeSHA256StreamsObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/public-bucket", "/private-bucket":
			if r.Method != http.MethodHead {
				t.Fatalf("unexpected bucket validation method: %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		case "/private-bucket/tenant-a/uploads/file.txt":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected object method: %s", r.Method)
			}
			_, _ = w.Write([]byte("hello world"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := newAdapterForTest(t, server.URL)
	hash, err := adapter.ComputeSHA256(context.Background(), pkgstorage.ObjectRef{
		Provider:   pkgstorage.ProviderS3,
		BucketName: "private-bucket",
		ObjectKey:  "tenant-a/uploads/file.txt",
	})
	if err != nil {
		t.Fatalf("ComputeSHA256 error = %v", err)
	}
	if hash != "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9" {
		t.Fatalf("hash = %q, want sha256(hello world)", hash)
	}
}

func TestPresignPutObjectReturnsURLAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()

	adapter := newAdapterForTest(t, server.URL)
	url, headers, err := adapter.PresignPutObject(context.Background(), pkgstorage.ObjectRef{
		Provider:   pkgstorage.ProviderS3,
		BucketName: "private-bucket",
		ObjectKey:  "tenant-a/uploads/file.txt",
	}, "text/plain", 0)
	if err != nil {
		t.Fatalf("PresignPutObject error = %v", err)
	}
	if !strings.Contains(url, "private-bucket/tenant-a/uploads/file.txt") {
		t.Fatalf("unexpected presigned url: %s", url)
	}
	if len(headers) == 0 {
		t.Fatal("expected signed headers")
	}
}

func TestPutObjectBuffersNonSeekableReader(t *testing.T) {
	payload := []byte("hello world")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/private-bucket/tenant-a/uploads/file.txt" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if string(data) != string(payload) {
			t.Fatalf("payload = %q, want %q", string(data), string(payload))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := newAdapterForTest(t, server.URL)
	err := adapter.PutObject(context.Background(), pkgstorage.ObjectRef{
		Provider:   pkgstorage.ProviderS3,
		BucketName: "private-bucket",
		ObjectKey:  "tenant-a/uploads/file.txt",
	}, "text/plain", onlyReader{r: bytes.NewBuffer(payload)}, int64(len(payload)))
	if err != nil {
		t.Fatalf("PutObject() error = %v", err)
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

func mustHex(t *testing.T, value string) []byte {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode hex %q: %v", value, err)
	}
	return decoded
}

type onlyReader struct {
	r io.Reader
}

func (o onlyReader) Read(p []byte) (int, error) {
	return o.r.Read(p)
}
