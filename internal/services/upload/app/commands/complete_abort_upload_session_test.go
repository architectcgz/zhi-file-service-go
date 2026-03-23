package commands_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	uploadtx "github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/tx"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestCompleteUploadSessionInlineCommitsMetadataAndOutbox(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 0, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "upload-inline-1",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "report.pdf",
		ContentType: "application/pdf",
		SizeBytes:   5,
		AccessLevel: storage.AccessLevelPrivate,
		Mode:        domain.SessionModeInline,
		Object: storage.ObjectRef{
			Provider:   storage.ProviderS3,
			BucketName: "private-bucket",
			ObjectKey:  "tenant-a/uploads/upload-inline-1/report.pdf",
		},
		CreatedAt: now.Add(-2 * time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(28 * time.Minute),
	})

	sessions := &stubCompleteSessionRepository{
		session: session,
		acquired: &ports.CompletionAcquireResult{
			Session:   session,
			Ownership: domain.CompletionOwnershipAcquired,
		},
		confirmed: session,
	}
	blobs := &stubBlobRepository{}
	files := &stubFileRepository{}
	usage := &stubTenantUsageRepository{}
	outbox := &stubUploadOutboxPublisher{}
	handler := commands.NewCompleteUploadSessionHandler(
		sessions,
		&stubSessionPartRepository{},
		blobs,
		files,
		&stubCompleteDedupRepository{},
		usage,
		outbox,
		&stubUploadTxManager{},
		&stubCompleteMultipartManager{},
		&stubObjectReader{
			metadata: storage.ObjectMetadata{
				SizeBytes:   5,
				ContentType: "application/pdf",
				ETag:        `"etag-inline"`,
			},
		},
		&stubSequenceIDGenerator{ids: []string{"blob-inline-1", "file-inline-1"}},
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-inline-1",
		IdempotencyKey:  "complete-inline-1",
		Auth:            newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.FileID != "file-inline-1" {
		t.Fatalf("file id = %q, want %q", result.FileID, "file-inline-1")
	}
	if result.UploadSession.Status != domain.SessionStatusCompleted {
		t.Fatalf("upload session status = %s, want %s", result.UploadSession.Status, domain.SessionStatusCompleted)
	}
	if blobs.upserted == nil || blobs.upserted.BlobID != "blob-inline-1" {
		t.Fatalf("unexpected blob upsert: %#v", blobs.upserted)
	}
	if files.created == nil || files.created.FileID != "file-inline-1" {
		t.Fatalf("unexpected file asset record: %#v", files.created)
	}
	if usage.deltaBytes != 5 {
		t.Fatalf("usage delta = %d, want 5", usage.deltaBytes)
	}
	if outbox.message == nil {
		t.Fatal("expected completed outbox message")
	}
	if outbox.message.EventType != "upload.session.completed.v1" {
		t.Fatalf("event type = %q, want %q", outbox.message.EventType, "upload.session.completed.v1")
	}
	payload := decodeOutboxPayload(t, outbox.message.Payload)
	if payload["fileId"] != "file-inline-1" || payload["blobObjectId"] != "blob-inline-1" {
		t.Fatalf("unexpected outbox payload: %#v", payload)
	}
}

func TestCompleteUploadSessionPresignedSingleUsesVerifiedHash(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 10, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "upload-single-1",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   9,
		AccessLevel: storage.AccessLevelPublic,
		Mode:        domain.SessionModePresignedSingle,
		Object: storage.ObjectRef{
			Provider:   storage.ProviderS3,
			BucketName: "public-bucket",
			ObjectKey:  "tenant-a/uploads/upload-single-1/avatar.png",
		},
		Hash:      &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt: now.Add(-2 * time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(28 * time.Minute),
	})

	blobs := &stubBlobRepository{}
	files := &stubFileRepository{}
	handler := commands.NewCompleteUploadSessionHandler(
		&stubCompleteSessionRepository{
			session: session,
			acquired: &ports.CompletionAcquireResult{
				Session:   session,
				Ownership: domain.CompletionOwnershipAcquired,
			},
			confirmed: session,
		},
		&stubSessionPartRepository{},
		blobs,
		files,
		&stubCompleteDedupRepository{},
		&stubTenantUsageRepository{},
		&stubUploadOutboxPublisher{},
		&stubUploadTxManager{},
		&stubCompleteMultipartManager{},
		&stubObjectReader{
			metadata: storage.ObjectMetadata{
				SizeBytes:   9,
				ContentType: "image/png",
				ETag:        `"etag-single"`,
				Checksum:    validHash(),
			},
		},
		&stubSequenceIDGenerator{ids: []string{"blob-single-1", "file-single-1"}},
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-single-1",
		IdempotencyKey:  "complete-single-1",
		Auth:            newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.FileID != "file-single-1" {
		t.Fatalf("file id = %q, want %q", result.FileID, "file-single-1")
	}
	if blobs.upserted == nil || blobs.upserted.Hash.Value != validHash() {
		t.Fatalf("unexpected blob record: %#v", blobs.upserted)
	}
	if files.created == nil || files.created.Hash == nil || files.created.Hash.Value != validHash() {
		t.Fatalf("unexpected file record: %#v", files.created)
	}
}

func TestCompleteUploadSessionRecordsDedupMetrics(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 12, 0, 0, time.UTC)

	tests := []struct {
		name         string
		decision     *domain.DedupDecision
		wantHit      int
		wantMiss     int
		generatorIDs []string
	}{
		{
			name: "dedup hit",
			decision: &domain.DedupDecision{
				Hit:    true,
				BlobID: "blob-hit-1",
				FileID: "file-existing-1",
			},
			wantHit:      1,
			wantMiss:     0,
			generatorIDs: []string{"file-hit-1"},
		},
		{
			name:         "dedup miss",
			decision:     nil,
			wantHit:      0,
			wantMiss:     1,
			generatorIDs: []string{"blob-miss-1", "file-miss-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := mustNewSession(t, domain.CreateSessionParams{
				ID:          "upload-single-dedup-1",
				TenantID:    "tenant-a",
				OwnerID:     "user-1",
				FileName:    "avatar.png",
				ContentType: "image/png",
				SizeBytes:   9,
				AccessLevel: storage.AccessLevelPublic,
				Mode:        domain.SessionModePresignedSingle,
				Object: storage.ObjectRef{
					Provider:   storage.ProviderS3,
					BucketName: "public-bucket",
					ObjectKey:  "tenant-a/uploads/upload-single-dedup-1/avatar.png",
				},
				Hash:      &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
				CreatedAt: now.Add(-2 * time.Minute),
				UpdatedAt: now.Add(-time.Minute),
				ExpiresAt: now.Add(28 * time.Minute),
			})

			metrics := &stubCompleteUploadMetrics{}
			handler := commands.NewCompleteUploadSessionHandler(
				&stubCompleteSessionRepository{
					session: session,
					acquired: &ports.CompletionAcquireResult{
						Session:   session,
						Ownership: domain.CompletionOwnershipAcquired,
					},
					confirmed: session,
				},
				&stubSessionPartRepository{},
				&stubBlobRepository{},
				&stubFileRepository{},
				&stubCompleteDedupRepository{decision: tt.decision},
				&stubTenantUsageRepository{},
				&stubUploadOutboxPublisher{},
				&stubUploadTxManager{},
				&stubCompleteMultipartManager{},
				&stubObjectReader{
					metadata: storage.ObjectMetadata{
						SizeBytes:   9,
						ContentType: "image/png",
						ETag:        `"etag-single-dedup"`,
						Checksum:    validHash(),
					},
				},
				&stubSequenceIDGenerator{ids: tt.generatorIDs},
				clock.NewFixed(now),
			).WithMetrics(metrics)

			if _, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
				UploadSessionID: session.ID,
				IdempotencyKey:  "complete-dedup-1",
				Auth:            newUploadAuth(),
			}); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if metrics.hitCount != tt.wantHit {
				t.Fatalf("hit count = %d, want %d", metrics.hitCount, tt.wantHit)
			}
			if metrics.missCount != tt.wantMiss {
				t.Fatalf("miss count = %d, want %d", metrics.missCount, tt.wantMiss)
			}
		})
	}
}

func TestCompleteUploadSessionFallsBackToStreamingHashWhenChecksumMissing(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 15, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "upload-single-hash-fallback-1",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   9,
		AccessLevel: storage.AccessLevelPublic,
		Mode:        domain.SessionModePresignedSingle,
		Object: storage.ObjectRef{
			Provider:   storage.ProviderS3,
			BucketName: "public-bucket",
			ObjectKey:  "tenant-a/uploads/upload-single-hash-fallback-1/avatar.png",
		},
		Hash:      &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt: now.Add(-2 * time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(28 * time.Minute),
	})

	reader := &stubObjectReader{
		metadata: storage.ObjectMetadata{
			SizeBytes:   9,
			ContentType: "image/png",
			ETag:        `"etag-single-fallback"`,
		},
		computedHash: validHash(),
	}
	handler := commands.NewCompleteUploadSessionHandler(
		&stubCompleteSessionRepository{
			session: session,
			acquired: &ports.CompletionAcquireResult{
				Session:   session,
				Ownership: domain.CompletionOwnershipAcquired,
			},
			confirmed: session,
		},
		&stubSessionPartRepository{},
		&stubBlobRepository{},
		&stubFileRepository{},
		&stubCompleteDedupRepository{},
		&stubTenantUsageRepository{},
		&stubUploadOutboxPublisher{},
		&stubUploadTxManager{},
		&stubCompleteMultipartManager{},
		reader,
		&stubSequenceIDGenerator{ids: []string{"blob-single-fallback-1", "file-single-fallback-1"}},
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-single-hash-fallback-1",
		IdempotencyKey:  "complete-single-fallback-1",
		Auth:            newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.FileID != "file-single-fallback-1" {
		t.Fatalf("file id = %q, want %q", result.FileID, "file-single-fallback-1")
	}
	if reader.computeCalls != 1 {
		t.Fatalf("compute calls = %d, want 1", reader.computeCalls)
	}
}

func TestCompleteUploadSessionDirectCompletesMultipartAndPersistsParts(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 20, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:               "upload-direct-1",
		TenantID:         "tenant-a",
		OwnerID:          "user-1",
		FileName:         "video.mp4",
		ContentType:      "video/mp4",
		SizeBytes:        200,
		AccessLevel:      storage.AccessLevelPrivate,
		Mode:             domain.SessionModeDirect,
		TotalParts:       2,
		Object:           storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/upload-direct-1/video.mp4"},
		ProviderUploadID: "provider-upload-1",
		Hash:             &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt:        now.Add(-2 * time.Minute),
		UpdatedAt:        now.Add(-time.Minute),
		ExpiresAt:        now.Add(28 * time.Minute),
	})
	partRepo := &stubSessionPartRepository{}
	multipart := &stubCompleteMultipartManager{
		parts: []storage.UploadedPart{
			{PartNumber: 1, ETag: `"etag-1"`, SizeBytes: 100},
			{PartNumber: 2, ETag: `"etag-2"`, SizeBytes: 100},
		},
	}
	handler := commands.NewCompleteUploadSessionHandler(
		&stubCompleteSessionRepository{
			session: session,
			acquired: &ports.CompletionAcquireResult{
				Session:   session,
				Ownership: domain.CompletionOwnershipAcquired,
			},
			confirmed: session,
		},
		partRepo,
		&stubBlobRepository{},
		&stubFileRepository{},
		&stubCompleteDedupRepository{},
		&stubTenantUsageRepository{},
		&stubUploadOutboxPublisher{},
		&stubUploadTxManager{},
		multipart,
		&stubObjectReader{
			metadata: storage.ObjectMetadata{
				SizeBytes:   200,
				ContentType: "video/mp4",
				ETag:        `"etag-final"`,
				Checksum:    validHash(),
			},
		},
		&stubSequenceIDGenerator{ids: []string{"blob-direct-1", "file-direct-1"}},
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-direct-1",
		IdempotencyKey:  "complete-direct-1",
		UploadedParts: []storage.UploadedPart{
			{PartNumber: 1, ETag: `"etag-1"`, SizeBytes: 100},
			{PartNumber: 2, ETag: `"etag-2"`, SizeBytes: 100},
		},
		Auth: newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if multipart.listCalls != 1 {
		t.Fatalf("list calls = %d, want 1", multipart.listCalls)
	}
	if multipart.completeCalls != 1 {
		t.Fatalf("complete calls = %d, want 1", multipart.completeCalls)
	}
	if partRepo.replaceCalls != 1 || len(partRepo.replaced) != 2 {
		t.Fatalf("unexpected persisted parts: %#v", partRepo.replaced)
	}
	if result.UploadSession.UploadedParts != 2 {
		t.Fatalf("uploaded parts = %d, want 2", result.UploadSession.UploadedParts)
	}
}

func TestCompleteUploadSessionReturnsExistingFileIDWhenCompleted(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 30, 0, 0, time.UTC)
	completedAt := now.Add(-time.Minute)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "upload-completed-1",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "done.pdf",
		ContentType: "application/pdf",
		SizeBytes:   10,
		AccessLevel: storage.AccessLevelPrivate,
		Mode:        domain.SessionModeInline,
		Status:      domain.SessionStatusCompleted,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/upload-completed-1/done.pdf"},
		FileID:      "file-existing-1",
		CreatedAt:   now.Add(-3 * time.Minute),
		UpdatedAt:   completedAt,
		CompletedAt: &completedAt,
		ExpiresAt:   now.Add(27 * time.Minute),
	})

	handler := commands.NewCompleteUploadSessionHandler(
		&stubCompleteSessionRepository{
			session: session,
			acquired: &ports.CompletionAcquireResult{
				Session:   session,
				Ownership: domain.CompletionOwnershipAlreadyDone,
			},
		},
		&stubSessionPartRepository{},
		&stubBlobRepository{},
		&stubFileRepository{},
		&stubCompleteDedupRepository{},
		&stubTenantUsageRepository{},
		&stubUploadOutboxPublisher{},
		&stubUploadTxManager{},
		&stubCompleteMultipartManager{},
		&stubObjectReader{},
		&stubSequenceIDGenerator{},
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-completed-1",
		IdempotencyKey:  "complete-existing-1",
		Auth:            newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.FileID != "file-existing-1" {
		t.Fatalf("file id = %q, want %q", result.FileID, "file-existing-1")
	}
}

func TestCompleteUploadSessionRejectsConcurrentCompletion(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 40, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "upload-busy-1",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "busy.pdf",
		ContentType: "application/pdf",
		SizeBytes:   10,
		AccessLevel: storage.AccessLevelPrivate,
		Mode:        domain.SessionModeInline,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/upload-busy-1/busy.pdf"},
		CreatedAt:   now.Add(-3 * time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(27 * time.Minute),
	})

	handler := commands.NewCompleteUploadSessionHandler(
		&stubCompleteSessionRepository{
			session: session,
			acquired: &ports.CompletionAcquireResult{
				Session:   session,
				Ownership: domain.CompletionOwnershipHeldByAnother,
			},
		},
		&stubSessionPartRepository{},
		&stubBlobRepository{},
		&stubFileRepository{},
		&stubCompleteDedupRepository{},
		&stubTenantUsageRepository{},
		&stubUploadOutboxPublisher{},
		&stubUploadTxManager{},
		&stubCompleteMultipartManager{},
		&stubObjectReader{},
		&stubSequenceIDGenerator{},
		clock.NewFixed(now),
	)

	_, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-busy-1",
		IdempotencyKey:  "complete-busy-1",
		Auth:            newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != xerrors.Code("UPLOAD_COMPLETE_IN_PROGRESS") {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.Code("UPLOAD_COMPLETE_IN_PROGRESS"), err)
	}
}

func TestCompleteUploadSessionRejectsHashMismatch(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 50, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "upload-hash-1",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   9,
		AccessLevel: storage.AccessLevelPublic,
		Mode:        domain.SessionModePresignedSingle,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "public-bucket", ObjectKey: "tenant-a/uploads/upload-hash-1/avatar.png"},
		Hash:        &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(28 * time.Minute),
	})
	files := &stubFileRepository{}
	handler := commands.NewCompleteUploadSessionHandler(
		&stubCompleteSessionRepository{
			session: session,
			acquired: &ports.CompletionAcquireResult{
				Session:   session,
				Ownership: domain.CompletionOwnershipAcquired,
			},
			confirmed: session,
		},
		&stubSessionPartRepository{},
		&stubBlobRepository{},
		files,
		&stubCompleteDedupRepository{},
		&stubTenantUsageRepository{},
		&stubUploadOutboxPublisher{},
		&stubUploadTxManager{},
		&stubCompleteMultipartManager{},
		&stubObjectReader{
			metadata: storage.ObjectMetadata{
				SizeBytes:   9,
				ContentType: "image/png",
				ETag:        `"etag-single"`,
				Checksum:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
		&stubSequenceIDGenerator{ids: []string{"blob-hash-1", "file-hash-1"}},
		clock.NewFixed(now),
	)

	_, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-hash-1",
		IdempotencyKey:  "complete-hash-1",
		Auth:            newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != xerrors.Code("UPLOAD_HASH_MISMATCH") {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.Code("UPLOAD_HASH_MISMATCH"), err)
	}
	if files.created != nil {
		t.Fatalf("expected no file creation, got %#v", files.created)
	}
}

func TestCompleteUploadSessionMarksFailedAndEnqueuesFailureEventOnHashMismatch(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 55, 0, 0, time.UTC)
	startedAt := now.Add(-time.Minute)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:                  "upload-hash-failed-1",
		TenantID:            "tenant-a",
		OwnerID:             "user-1",
		FileName:            "avatar.png",
		ContentType:         "image/png",
		SizeBytes:           9,
		AccessLevel:         storage.AccessLevelPublic,
		Mode:                domain.SessionModePresignedSingle,
		Status:              domain.SessionStatusCompleting,
		Object:              storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "public-bucket", ObjectKey: "tenant-a/uploads/upload-hash-failed-1/avatar.png"},
		Hash:                &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CompletionToken:     "complete-hash-failed-1",
		CompletionStartedAt: &startedAt,
		CreatedAt:           now.Add(-2 * time.Minute),
		UpdatedAt:           startedAt,
		ExpiresAt:           now.Add(28 * time.Minute),
	})
	sessions := &stubCompleteSessionRepository{
		session: session,
		acquired: &ports.CompletionAcquireResult{
			Session:   session,
			Ownership: domain.CompletionOwnershipHeldByCaller,
		},
		confirmed: session,
	}
	outbox := &stubUploadOutboxPublisher{}
	handler := commands.NewCompleteUploadSessionHandler(
		sessions,
		&stubSessionPartRepository{},
		&stubBlobRepository{},
		&stubFileRepository{},
		&stubCompleteDedupRepository{},
		&stubTenantUsageRepository{},
		outbox,
		&stubUploadTxManager{},
		&stubCompleteMultipartManager{},
		&stubObjectReader{
			metadata: storage.ObjectMetadata{
				SizeBytes:   9,
				ContentType: "image/png",
				ETag:        `"etag-single"`,
				Checksum:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
		&stubSequenceIDGenerator{},
		clock.NewFixed(now),
	)

	_, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-hash-failed-1",
		IdempotencyKey:  "complete-hash-failed-1",
		RequestID:       "req-hash-failed-1",
		Auth:            newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != xerrors.Code("UPLOAD_HASH_MISMATCH") {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.Code("UPLOAD_HASH_MISMATCH"), err)
	}
	if sessions.saved == nil || sessions.saved.Status != domain.SessionStatusFailed {
		t.Fatalf("expected failed session to be saved, got %#v", sessions.saved)
	}
	if sessions.saved.FailureCode != "UPLOAD_HASH_MISMATCH" {
		t.Fatalf("failure code = %q, want %q", sessions.saved.FailureCode, "UPLOAD_HASH_MISMATCH")
	}
	if outbox.message == nil {
		t.Fatal("expected failed outbox message")
	}
	if outbox.message.EventType != "upload.session.failed.v1" {
		t.Fatalf("event type = %q, want %q", outbox.message.EventType, "upload.session.failed.v1")
	}
	payload := decodeOutboxPayload(t, outbox.message.Payload)
	if payload["failureCode"] != "UPLOAD_HASH_MISMATCH" {
		t.Fatalf("unexpected outbox payload: %#v", payload)
	}
}

func TestCompleteUploadSessionRetriesWhenCallerStillOwnsCompletion(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 57, 0, 0, time.UTC)
	startedAt := now.Add(-time.Minute)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:                  "upload-held-1",
		TenantID:            "tenant-a",
		OwnerID:             "user-1",
		FileName:            "avatar.png",
		ContentType:         "image/png",
		SizeBytes:           9,
		AccessLevel:         storage.AccessLevelPublic,
		Mode:                domain.SessionModePresignedSingle,
		Status:              domain.SessionStatusCompleting,
		Object:              storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "public-bucket", ObjectKey: "tenant-a/uploads/upload-held-1/avatar.png"},
		Hash:                &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CompletionToken:     "complete-held-1",
		CompletionStartedAt: &startedAt,
		CreatedAt:           now.Add(-2 * time.Minute),
		UpdatedAt:           startedAt,
		ExpiresAt:           now.Add(28 * time.Minute),
	})
	handler := commands.NewCompleteUploadSessionHandler(
		&stubCompleteSessionRepository{
			session: session,
			acquired: &ports.CompletionAcquireResult{
				Session:   session,
				Ownership: domain.CompletionOwnershipHeldByCaller,
			},
			confirmed: session,
		},
		&stubSessionPartRepository{},
		&stubBlobRepository{},
		&stubFileRepository{},
		&stubCompleteDedupRepository{},
		&stubTenantUsageRepository{},
		&stubUploadOutboxPublisher{},
		&stubUploadTxManager{},
		&stubCompleteMultipartManager{},
		&stubObjectReader{
			metadata: storage.ObjectMetadata{
				SizeBytes:   9,
				ContentType: "image/png",
				ETag:        `"etag-single"`,
				Checksum:    validHash(),
			},
		},
		&stubSequenceIDGenerator{ids: []string{"blob-held-1", "file-held-1"}},
		clock.NewFixed(now),
	)

	result, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-held-1",
		IdempotencyKey:  "complete-held-1",
		Auth:            newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.FileID != "file-held-1" {
		t.Fatalf("file id = %q, want %q", result.FileID, "file-held-1")
	}
	if result.UploadSession.Status != domain.SessionStatusCompleted {
		t.Fatalf("status = %s, want %s", result.UploadSession.Status, domain.SessionStatusCompleted)
	}
}

func TestCompleteUploadSessionLeavesSessionCompletingOnTransientStorageError(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 58, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "upload-transient-1",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   9,
		AccessLevel: storage.AccessLevelPublic,
		Mode:        domain.SessionModePresignedSingle,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "public-bucket", ObjectKey: "tenant-a/uploads/upload-transient-1/avatar.png"},
		Hash:        &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(28 * time.Minute),
	})
	sessions := &stubCompleteSessionRepository{
		session: session,
		acquired: &ports.CompletionAcquireResult{
			Session:   session,
			Ownership: domain.CompletionOwnershipAcquired,
		},
		confirmed: session,
	}
	outbox := &stubUploadOutboxPublisher{}
	handler := commands.NewCompleteUploadSessionHandler(
		sessions,
		&stubSessionPartRepository{},
		&stubBlobRepository{},
		&stubFileRepository{},
		&stubCompleteDedupRepository{},
		&stubTenantUsageRepository{},
		outbox,
		&stubUploadTxManager{},
		&stubCompleteMultipartManager{},
		&stubObjectReader{err: errors.New("object storage unavailable")},
		&stubSequenceIDGenerator{},
		clock.NewFixed(now),
	)

	_, err := handler.Handle(context.Background(), commands.CompleteUploadSessionCommand{
		UploadSessionID: "upload-transient-1",
		IdempotencyKey:  "complete-transient-1",
		Auth:            newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != xerrors.CodeServiceUnavailable {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeServiceUnavailable, err)
	}
	if sessions.saved != nil {
		t.Fatalf("expected no failed save on transient error, got %#v", sessions.saved)
	}
	if outbox.message != nil {
		t.Fatalf("expected no outbox message on transient error, got %#v", outbox.message)
	}
	if session.Status != domain.SessionStatusCompleting {
		t.Fatalf("status = %s, want %s", session.Status, domain.SessionStatusCompleting)
	}
}

func TestAbortUploadSessionMarksSessionAborted(t *testing.T) {
	now := time.Date(2026, 3, 22, 19, 0, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "upload-abort-inline-1",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "draft.pdf",
		ContentType: "application/pdf",
		SizeBytes:   10,
		AccessLevel: storage.AccessLevelPrivate,
		Mode:        domain.SessionModeInline,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/upload-abort-inline-1/draft.pdf"},
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(28 * time.Minute),
	})
	sessions := &stubAbortSessionRepository{session: session}
	handler := commands.NewAbortUploadSessionHandler(sessions, &stubCompleteMultipartManager{}, clock.NewFixed(now))

	result, err := handler.Handle(context.Background(), commands.AbortUploadSessionCommand{
		UploadSessionID: "upload-abort-inline-1",
		Auth:            newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.Status != domain.SessionStatusAborted {
		t.Fatalf("status = %s, want %s", result.Status, domain.SessionStatusAborted)
	}
	if sessions.saved == nil || sessions.saved.Status != domain.SessionStatusAborted {
		t.Fatalf("unexpected saved session: %#v", sessions.saved)
	}
}

func TestAbortUploadSessionDirectSwallowsProviderAbortFailure(t *testing.T) {
	now := time.Date(2026, 3, 22, 19, 10, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:               "upload-abort-direct-1",
		TenantID:         "tenant-a",
		OwnerID:          "user-1",
		FileName:         "video.mp4",
		ContentType:      "video/mp4",
		SizeBytes:        10,
		AccessLevel:      storage.AccessLevelPrivate,
		Mode:             domain.SessionModeDirect,
		TotalParts:       2,
		Object:           storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/upload-abort-direct-1/video.mp4"},
		ProviderUploadID: "provider-upload-1",
		Hash:             &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt:        now.Add(-2 * time.Minute),
		UpdatedAt:        now.Add(-time.Minute),
		ExpiresAt:        now.Add(28 * time.Minute),
	})
	multipart := &stubCompleteMultipartManager{abortErr: errors.New("provider down")}
	handler := commands.NewAbortUploadSessionHandler(&stubAbortSessionRepository{session: session}, multipart, clock.NewFixed(now))

	result, err := handler.Handle(context.Background(), commands.AbortUploadSessionCommand{
		UploadSessionID: "upload-abort-direct-1",
		Auth:            newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.Status != domain.SessionStatusAborted {
		t.Fatalf("status = %s, want %s", result.Status, domain.SessionStatusAborted)
	}
	if multipart.abortCalls != 1 {
		t.Fatalf("abort calls = %d, want 1", multipart.abortCalls)
	}
}

func TestAbortUploadSessionRejectsCompletedSession(t *testing.T) {
	now := time.Date(2026, 3, 22, 19, 20, 0, 0, time.UTC)
	completedAt := now.Add(-time.Minute)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:          "upload-abort-completed-1",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "done.pdf",
		ContentType: "application/pdf",
		SizeBytes:   10,
		AccessLevel: storage.AccessLevelPrivate,
		Mode:        domain.SessionModeInline,
		Status:      domain.SessionStatusCompleted,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/upload-abort-completed-1/done.pdf"},
		FileID:      "file-completed-1",
		CreatedAt:   now.Add(-3 * time.Minute),
		UpdatedAt:   completedAt,
		CompletedAt: &completedAt,
		ExpiresAt:   now.Add(27 * time.Minute),
	})
	handler := commands.NewAbortUploadSessionHandler(&stubAbortSessionRepository{session: session}, &stubCompleteMultipartManager{}, clock.NewFixed(now))

	_, err := handler.Handle(context.Background(), commands.AbortUploadSessionCommand{
		UploadSessionID: "upload-abort-completed-1",
		Auth:            newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != domain.CodeUploadSessionStateConflict {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, domain.CodeUploadSessionStateConflict, err)
	}
}

type stubCompleteSessionRepository struct {
	session        *domain.Session
	getErr         error
	acquired       *ports.CompletionAcquireResult
	acquireErr     error
	acquireRequest ports.CompletionAcquireRequest
	confirmed      *domain.Session
	confirmErr     error
	confirmToken   string
	saved          *domain.Session
	saveErr        error
}

func (s *stubCompleteSessionRepository) Create(context.Context, *domain.Session) error {
	panic("unexpected call")
}

func (s *stubCompleteSessionRepository) Save(_ context.Context, session *domain.Session) error {
	s.saved = session
	return s.saveErr
}

func (s *stubCompleteSessionRepository) GetByID(context.Context, string, string) (*domain.Session, error) {
	return s.session, s.getErr
}

func (s *stubCompleteSessionRepository) FindReusable(context.Context, ports.ReusableSessionQuery) (*domain.Session, error) {
	panic("unexpected call")
}

func (s *stubCompleteSessionRepository) AcquireCompletion(_ context.Context, request ports.CompletionAcquireRequest) (*ports.CompletionAcquireResult, error) {
	s.acquireRequest = request
	return s.acquired, s.acquireErr
}

func (s *stubCompleteSessionRepository) ConfirmCompletionOwner(_ context.Context, _ string, _ string, completionToken string) (*domain.Session, error) {
	s.confirmToken = completionToken
	return s.confirmed, s.confirmErr
}

type stubSessionPartRepository struct {
	replaced     []ports.SessionPartRecord
	replaceCalls int
}

func (s *stubSessionPartRepository) ListBySessionID(context.Context, string, string) ([]ports.SessionPartRecord, error) {
	return nil, nil
}

func (s *stubSessionPartRepository) Upsert(context.Context, ports.SessionPartRecord) error {
	panic("unexpected call")
}

func (s *stubSessionPartRepository) Replace(_ context.Context, _ string, _ string, records []ports.SessionPartRecord) error {
	s.replaceCalls++
	s.replaced = append([]ports.SessionPartRecord(nil), records...)
	return nil
}

type stubBlobRepository struct {
	upserted     *ports.BlobRecord
	adjustedBlob string
	adjustedBy   int64
}

func (s *stubBlobRepository) Upsert(_ context.Context, record ports.BlobRecord) error {
	copied := record
	s.upserted = &copied
	return nil
}

func (s *stubBlobRepository) AdjustReferenceCount(_ context.Context, blobID string, delta int64) error {
	s.adjustedBlob = blobID
	s.adjustedBy = delta
	return nil
}

type stubFileRepository struct {
	created *ports.FileAssetRecord
}

func (s *stubFileRepository) CreateFileAsset(_ context.Context, record ports.FileAssetRecord) error {
	copied := record
	s.created = &copied
	return nil
}

type stubCompleteDedupRepository struct {
	decision  *domain.DedupDecision
	lookupKey ports.DedupLookupKey
}

func (s *stubCompleteDedupRepository) LookupByHash(_ context.Context, key ports.DedupLookupKey) (*domain.DedupDecision, error) {
	s.lookupKey = key
	return s.decision, nil
}

func (s *stubCompleteDedupRepository) ClaimHash(context.Context, ports.DedupClaim) error {
	panic("unexpected call")
}

type stubCompleteUploadMetrics struct {
	hitCount  int
	missCount int
}

func (s *stubCompleteUploadMetrics) RecordDedupHit() {
	s.hitCount++
}

func (s *stubCompleteUploadMetrics) RecordDedupMiss() {
	s.missCount++
}

type stubTenantUsageRepository struct {
	tenantID   string
	deltaBytes int64
}

func (s *stubTenantUsageRepository) ApplyDelta(_ context.Context, tenantID string, deltaBytes int64) error {
	s.tenantID = tenantID
	s.deltaBytes = deltaBytes
	return nil
}

type stubUploadOutboxPublisher struct {
	message *ports.OutboxMessage
}

func (s *stubUploadOutboxPublisher) Enqueue(_ context.Context, message ports.OutboxMessage) error {
	copied := message
	copied.Payload = append([]byte(nil), message.Payload...)
	s.message = &copied
	return nil
}

type stubUploadTxManager struct {
	calls int
}

func (s *stubUploadTxManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	s.calls++
	return fn(ctx)
}

var _ uploadtx.Manager = (*stubUploadTxManager)(nil)

type stubObjectReader struct {
	metadata     storage.ObjectMetadata
	err          error
	computedHash string
	computeErr   error
	computeCalls int
}

func (s *stubObjectReader) HeadObject(context.Context, storage.ObjectRef) (storage.ObjectMetadata, error) {
	return s.metadata, s.err
}

func (s *stubObjectReader) ComputeSHA256(context.Context, storage.ObjectRef) (string, error) {
	s.computeCalls++
	return s.computedHash, s.computeErr
}

type stubCompleteMultipartManager struct {
	parts         []storage.UploadedPart
	listCalls     int
	completeCalls int
	abortCalls    int
	abortErr      error
}

func (s *stubCompleteMultipartManager) CreateMultipartUpload(context.Context, storage.ObjectRef, string) (string, error) {
	panic("unexpected call")
}

func (s *stubCompleteMultipartManager) UploadPart(context.Context, storage.ObjectRef, string, int, io.Reader, int64) (string, error) {
	panic("unexpected call")
}

func (s *stubCompleteMultipartManager) ListUploadedParts(context.Context, storage.ObjectRef, string) ([]storage.UploadedPart, error) {
	s.listCalls++
	return append([]storage.UploadedPart(nil), s.parts...), nil
}

func (s *stubCompleteMultipartManager) CompleteMultipartUpload(context.Context, storage.ObjectRef, string, []storage.UploadedPart) error {
	s.completeCalls++
	return nil
}

func (s *stubCompleteMultipartManager) AbortMultipartUpload(context.Context, storage.ObjectRef, string) error {
	s.abortCalls++
	return s.abortErr
}

type stubSequenceIDGenerator struct {
	ids   []string
	calls int
}

func (s *stubSequenceIDGenerator) New() (string, error) {
	if s.calls >= len(s.ids) {
		return "", errors.New("no more ids")
	}
	id := s.ids[s.calls]
	s.calls++
	return id, nil
}

type stubAbortSessionRepository struct {
	session *domain.Session
	saved   *domain.Session
	err     error
	saveErr error
}

func (s *stubAbortSessionRepository) Create(context.Context, *domain.Session) error {
	panic("unexpected call")
}

func (s *stubAbortSessionRepository) Save(_ context.Context, session *domain.Session) error {
	s.saved = session
	return s.saveErr
}

func (s *stubAbortSessionRepository) GetByID(context.Context, string, string) (*domain.Session, error) {
	return s.session, s.err
}

func (s *stubAbortSessionRepository) FindReusable(context.Context, ports.ReusableSessionQuery) (*domain.Session, error) {
	panic("unexpected call")
}

func (s *stubAbortSessionRepository) AcquireCompletion(context.Context, ports.CompletionAcquireRequest) (*ports.CompletionAcquireResult, error) {
	panic("unexpected call")
}

func (s *stubAbortSessionRepository) ConfirmCompletionOwner(context.Context, string, string, string) (*domain.Session, error) {
	panic("unexpected call")
}

func decodeOutboxPayload(t *testing.T, payload []byte) map[string]any {
	t.Helper()

	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	return value
}
