package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/commands"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestPresignMultipartPartsReturnsSortedPartURLs(t *testing.T) {
	now := time.Date(2026, 3, 22, 16, 30, 0, 0, time.UTC)
	session := mustNewSession(t, domain.CreateSessionParams{
		ID:               "01JQ5CF8A4R1F89YC4SYRPQF8R",
		TenantID:         "tenant-a",
		OwnerID:          "user-1",
		FileName:         "video.mp4",
		ContentType:      "video/mp4",
		SizeBytes:        1024,
		AccessLevel:      storage.AccessLevelPrivate,
		Mode:             domain.SessionModeDirect,
		TotalParts:       3,
		Object:           storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/01JQ5CF8A4R1F89YC4SYRPQF8R/video.mp4"},
		ProviderUploadID: "upload-123",
		Hash:             &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt:        now.Add(-time.Minute),
		UpdatedAt:        now.Add(-time.Minute),
		ExpiresAt:        now.Add(29 * time.Minute),
	})
	sessions := &stubInlineSessionRepository{session: session}
	presign := &stubPresignManager{
		url: "https://storage.example.com/part",
		headers: map[string]string{
			"x-amz-content-sha256": "UNSIGNED-PAYLOAD",
		},
	}
	handler := commands.NewPresignMultipartPartsHandler(
		sessions,
		presign,
		clock.NewFixed(now),
		commands.PresignMultipartPartsConfig{
			DefaultTTL: 15 * time.Minute,
			MaxTTL:     24 * time.Hour,
		},
	)

	result, err := handler.Handle(context.Background(), commands.PresignMultipartPartsCommand{
		UploadSessionID: session.ID,
		ExpiresIn:       10 * time.Minute,
		Parts: []commands.PresignMultipartPart{
			{PartNumber: 3},
			{PartNumber: 1},
			{PartNumber: 2},
		},
		Auth: newUploadAuth(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(result.Parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(result.Parts))
	}
	if result.Parts[0].PartNumber != 1 || result.Parts[2].PartNumber != 3 {
		t.Fatalf("unexpected part order: %#v", result.Parts)
	}
	if !result.Parts[0].ExpiresAt.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("unexpected expires at: %s", result.Parts[0].ExpiresAt)
	}
	if presign.uploadPartCalls != 3 {
		t.Fatalf("presign calls = %d, want 3", presign.uploadPartCalls)
	}
}

func TestPresignMultipartPartsRejectsInvalidModeAndState(t *testing.T) {
	now := time.Date(2026, 3, 22, 16, 40, 0, 0, time.UTC)
	presignedSingle := mustNewSession(t, domain.CreateSessionParams{
		ID:          "01JQ5CKK9Z9SE1XSYF8Y0K6FTR",
		TenantID:    "tenant-a",
		OwnerID:     "user-1",
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   10,
		AccessLevel: storage.AccessLevelPublic,
		Mode:        domain.SessionModePresignedSingle,
		Object:      storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "public-bucket", ObjectKey: "tenant-a/uploads/01JQ5CKK9Z9SE1XSYF8Y0K6FTR/avatar.png"},
		Hash:        &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(29 * time.Minute),
	})
	handler := commands.NewPresignMultipartPartsHandler(&stubInlineSessionRepository{session: presignedSingle}, &stubPresignManager{}, clock.NewFixed(now), commands.PresignMultipartPartsConfig{})
	_, err := handler.Handle(context.Background(), commands.PresignMultipartPartsCommand{
		UploadSessionID: presignedSingle.ID,
		Parts:           []commands.PresignMultipartPart{{PartNumber: 1}},
		Auth:            newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != domain.CodeUploadModeInvalid {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, domain.CodeUploadModeInvalid, err)
	}

	completed := mustNewSession(t, domain.CreateSessionParams{
		ID:               "01JQ5CPBV93M1FM0WZSBH6FADP",
		TenantID:         "tenant-a",
		OwnerID:          "user-1",
		FileName:         "video.mp4",
		ContentType:      "video/mp4",
		SizeBytes:        10,
		AccessLevel:      storage.AccessLevelPrivate,
		Mode:             domain.SessionModeDirect,
		TotalParts:       2,
		Object:           storage.ObjectRef{Provider: storage.ProviderS3, BucketName: "private-bucket", ObjectKey: "tenant-a/uploads/01JQ5CPBV93M1FM0WZSBH6FADP/video.mp4"},
		ProviderUploadID: "upload-123",
		Hash:             &domain.ContentHash{Algorithm: "SHA256", Value: validHash()},
		Status:           domain.SessionStatusCompleted,
		FileID:           "file-1",
		CreatedAt:        now.Add(-2 * time.Minute),
		UpdatedAt:        now.Add(-time.Minute),
		CompletedAt:      timePtr(now.Add(-time.Minute)),
		ExpiresAt:        now.Add(28 * time.Minute),
	})
	handler = commands.NewPresignMultipartPartsHandler(&stubInlineSessionRepository{session: completed}, &stubPresignManager{}, clock.NewFixed(now), commands.PresignMultipartPartsConfig{})
	_, err = handler.Handle(context.Background(), commands.PresignMultipartPartsCommand{
		UploadSessionID: completed.ID,
		Parts:           []commands.PresignMultipartPart{{PartNumber: 1}},
		Auth:            newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != domain.CodeUploadSessionStateConflict {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, domain.CodeUploadSessionStateConflict, err)
	}
}

func TestPresignMultipartPartsRejectsDuplicatePartNumbers(t *testing.T) {
	handler := commands.NewPresignMultipartPartsHandler(&stubInlineSessionRepository{}, &stubPresignManager{}, clock.NewFixed(time.Date(2026, 3, 22, 16, 50, 0, 0, time.UTC)), commands.PresignMultipartPartsConfig{})

	_, err := handler.Handle(context.Background(), commands.PresignMultipartPartsCommand{
		UploadSessionID: "01JQ5CRMSQ72YMQMTV7WWKBMD6",
		Parts: []commands.PresignMultipartPart{
			{PartNumber: 1},
			{PartNumber: 1},
		},
		Auth: newUploadAuth(),
	})
	if code := xerrors.CodeOf(err); code != xerrors.CodeInvalidArgument {
		t.Fatalf("CodeOf() = %q, want %q (err=%v)", code, xerrors.CodeInvalidArgument, err)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
