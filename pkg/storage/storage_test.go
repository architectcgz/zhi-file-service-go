package storage

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestAccessLevel_Values(t *testing.T) {
	t.Parallel()

	if AccessLevelPublic != "PUBLIC" {
		t.Fatalf("AccessLevelPublic = %q, want %q", AccessLevelPublic, "PUBLIC")
	}
	if AccessLevelPrivate != "PRIVATE" {
		t.Fatalf("AccessLevelPrivate = %q, want %q", AccessLevelPrivate, "PRIVATE")
	}
}

func TestCanonicalErrors_AreDistinct(t *testing.T) {
	t.Parallel()

	if errors.Is(ErrObjectNotFound, ErrMultipartNotFound) {
		t.Fatalf("ErrObjectNotFound should not match ErrMultipartNotFound")
	}
	if ErrProviderUnavailable.Error() == "" {
		t.Fatalf("ErrProviderUnavailable should have non-empty message")
	}
}

func TestInterfaces_CompileContracts(t *testing.T) {
	t.Parallel()

	requiresBucketResolver(stubBucketResolver{})
	requiresObjectLocator(stubObjectLocator{})
	requiresObjectReader(stubObjectReader{})
	requiresObjectWriter(stubObjectWriter{})
	requiresInlineObjectWriter(stubInlineObjectWriter{})
	requiresMultipartManager(stubMultipartManager{})
	requiresPresignManager(stubPresignManager{})
}

func requiresBucketResolver(BucketResolver) {}
func requiresObjectLocator(ObjectLocator)   {}
func requiresObjectReader(ObjectReader)     {}
func requiresObjectWriter(ObjectWriter)     {}
func requiresInlineObjectWriter(InlineObjectWriter) {
}
func requiresMultipartManager(MultipartManager) {}
func requiresPresignManager(PresignManager)     {}

type stubBucketResolver struct{}

func (stubBucketResolver) Resolve(AccessLevel) (BucketRef, error) { return BucketRef{}, nil }
func (stubBucketResolver) Normalize(bucketName string) string     { return bucketName }

type stubObjectLocator struct{}

func (stubObjectLocator) ResolveObjectURL(ObjectRef) (string, error) { return "", nil }

type stubObjectReader struct{}

func (stubObjectReader) HeadObject(context.Context, ObjectRef) (ObjectMetadata, error) {
	return ObjectMetadata{}, nil
}

type stubObjectWriter struct{}

func (stubObjectWriter) DeleteObject(context.Context, ObjectRef) error                 { return nil }
func (stubObjectWriter) CopyObject(context.Context, ObjectRef, ObjectRef) error        { return nil }

type stubInlineObjectWriter struct{}

func (stubInlineObjectWriter) PutObject(context.Context, ObjectRef, string, io.Reader, int64) error {
	return nil
}

type stubMultipartManager struct{}

func (stubMultipartManager) CreateMultipartUpload(context.Context, ObjectRef, string) (string, error) {
	return "upload-id", nil
}
func (stubMultipartManager) UploadPart(context.Context, ObjectRef, string, int, io.Reader, int64) (string, error) {
	return "etag", nil
}
func (stubMultipartManager) ListUploadedParts(context.Context, ObjectRef, string) ([]UploadedPart, error) {
	return nil, nil
}
func (stubMultipartManager) CompleteMultipartUpload(context.Context, ObjectRef, string, []UploadedPart) error {
	return nil
}
func (stubMultipartManager) AbortMultipartUpload(context.Context, ObjectRef, string) error { return nil }

type stubPresignManager struct{}

func (stubPresignManager) PresignPutObject(context.Context, ObjectRef, string, time.Duration) (string, map[string]string, error) {
	return "https://example.com/put", nil, nil
}
func (stubPresignManager) PresignUploadPart(context.Context, ObjectRef, string, int, time.Duration) (string, map[string]string, error) {
	return "https://example.com/part", nil, nil
}
func (stubPresignManager) PresignGetObject(context.Context, ObjectRef, time.Duration) (string, error) {
	return "https://example.com/get", nil
}
