package domain

import (
	"testing"
	"time"

	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

func TestNewSessionRejectsMissingOwnerID(t *testing.T) {
	params := validSessionParams(SessionModeInline)
	params.OwnerID = ""

	_, err := NewSession(params)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNewSessionRejectsMissingHashForDirectMode(t *testing.T) {
	_, err := NewSession(validSessionParams(SessionModeDirect))
	if xerrors.CodeOf(err) != CodeUploadHashRequired {
		t.Fatalf("expected upload hash required, got: %v", err)
	}
}

func TestNewSessionRejectsProviderUploadIDForInlineMode(t *testing.T) {
	params := validSessionParams(SessionModeInline)
	params.ProviderUploadID = "upload-1"

	_, err := NewSession(params)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNewSessionAcceptsDirectModeWithHashAndProviderUploadID(t *testing.T) {
	params := validSessionParams(SessionModeDirect)
	params.Hash = &ContentHash{Algorithm: "sha256", Value: validSHA256}
	params.ProviderUploadID = "upload-1"
	params.TotalParts = 3

	session, err := NewSession(params)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if session.TotalParts != 3 {
		t.Fatalf("unexpected total parts: %d", session.TotalParts)
	}
}

func TestSessionMarkUploadingAndComplete(t *testing.T) {
	params := validSessionParams(SessionModeInline)
	session, err := NewSession(params)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	if err := session.MarkUploading(1); err != nil {
		t.Fatalf("MarkUploading returned error: %v", err)
	}
	if session.Status != SessionStatusUploading {
		t.Fatalf("unexpected status after uploading: %s", session.Status)
	}
	if session.CompletedParts != 1 {
		t.Fatalf("unexpected completed parts: %d", session.CompletedParts)
	}

	ownership, err := session.AcquireCompletion("token-1", params.CreatedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("AcquireCompletion returned error: %v", err)
	}
	if ownership != CompletionOwnershipAcquired {
		t.Fatalf("unexpected ownership result: %s", ownership)
	}
	if session.Status != SessionStatusCompleting {
		t.Fatalf("unexpected status after acquire completion: %s", session.Status)
	}
	if session.CompletionToken != "token-1" || session.CompletionStartedAt == nil {
		t.Fatalf("completion ownership metadata not captured: %#v", session)
	}

	if err := session.MarkCompleted("file-1", params.CreatedAt.Add(2*time.Minute)); err != nil {
		t.Fatalf("MarkCompleted returned error: %v", err)
	}
	if session.Status != SessionStatusCompleted || session.FileID != "file-1" {
		t.Fatalf("unexpected completed session: %#v", session)
	}
}

func TestSessionAbortRejectsCompletedSession(t *testing.T) {
	params := validSessionParams(SessionModeInline)
	params.Status = SessionStatusCompleted
	params.FileID = "file-1"
	completedAt := params.CreatedAt.Add(time.Minute)
	params.CompletedAt = &completedAt

	session, err := NewSession(params)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	err = session.Abort(params.CreatedAt.Add(time.Minute))
	if xerrors.CodeOf(err) != CodeUploadSessionStateConflict {
		t.Fatalf("expected state conflict, got: %v", err)
	}
}

func TestSessionAcquireCompletionDistinguishesOwnerStates(t *testing.T) {
	params := validSessionParams(SessionModeInline)
	session, err := NewSession(params)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	acquired, err := session.AcquireCompletion("token-1", params.CreatedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("AcquireCompletion returned error: %v", err)
	}
	if acquired != CompletionOwnershipAcquired || !acquired.OwnsCompletion() {
		t.Fatalf("unexpected acquired ownership: %s", acquired)
	}

	heldByCaller, err := session.AcquireCompletion("token-1", params.CreatedAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("AcquireCompletion same token returned error: %v", err)
	}
	if heldByCaller != CompletionOwnershipHeldByCaller || !heldByCaller.OwnsCompletion() {
		t.Fatalf("unexpected same-token ownership: %s", heldByCaller)
	}

	heldByAnother, err := session.AcquireCompletion("token-2", params.CreatedAt.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("AcquireCompletion different token returned error: %v", err)
	}
	if heldByAnother != CompletionOwnershipHeldByAnother || heldByAnother.OwnsCompletion() {
		t.Fatalf("unexpected different-token ownership: %s", heldByAnother)
	}
}

func TestSessionAcquireCompletionRequiresOwnershipMetadata(t *testing.T) {
	params := validSessionParams(SessionModeInline)
	session, err := NewSession(params)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	_, err = session.AcquireCompletion("", time.Time{})
	if xerrors.CodeOf(err) != CodeUploadCompletionOwnershipInvalid {
		t.Fatalf("expected invalid completion ownership, got: %v", err)
	}
}

func TestNewSessionRejectsCompletingWithoutOwnershipMetadata(t *testing.T) {
	params := validSessionParams(SessionModeInline)
	params.Status = SessionStatusCompleting

	_, err := NewSession(params)
	if xerrors.CodeOf(err) != CodeUploadCompletionOwnershipInvalid {
		t.Fatalf("expected invalid completion ownership, got: %v", err)
	}
}

func TestSessionMarkFailedCapturesFailureFacts(t *testing.T) {
	params := validSessionParams(SessionModeInline)
	session, err := NewSession(params)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	if _, err := session.AcquireCompletion("token-1", params.CreatedAt.Add(time.Minute)); err != nil {
		t.Fatalf("AcquireCompletion returned error: %v", err)
	}
	if err := session.MarkFailed("OBJECT_MISSING", "object metadata is missing", params.CreatedAt.Add(2*time.Minute)); err != nil {
		t.Fatalf("MarkFailed returned error: %v", err)
	}
	if session.Status != SessionStatusFailed {
		t.Fatalf("unexpected status after failure: %s", session.Status)
	}
	if session.FailureCode != "OBJECT_MISSING" || session.FailureMessage == "" || session.FailedAt == nil {
		t.Fatalf("failure facts not captured: %#v", session)
	}
}

func validSessionParams(mode SessionMode) CreateSessionParams {
	now := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	return CreateSessionParams{
		ID:          "upload-session-1",
		TenantID:    "tenant-1",
		OwnerID:     "user-1",
		FileName:    "invoice.pdf",
		ContentType: "application/pdf",
		SizeBytes:   1024,
		AccessLevel: pkgstorage.AccessLevelPrivate,
		Mode:        mode,
		TotalParts:  1,
		Object: pkgstorage.ObjectRef{
			Provider:   pkgstorage.ProviderS3,
			BucketName: "private-bucket",
			ObjectKey:  "tenant-1/upload-session-1",
		},
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}
}

const validSHA256 = "4f6f0d53c1efb6bb7c9f6b4e5b7d7e2b7b5b2f4b33f3ef0d4ec2ef9f74de4f75"
