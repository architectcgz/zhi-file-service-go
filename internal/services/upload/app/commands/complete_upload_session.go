package commands

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/tx"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const uploadSessionCompletedEventType = "upload.session.completed.v1"

type CompleteUploadSessionCommand struct {
	UploadSessionID string
	UploadedParts   []pkgstorage.UploadedPart
	ContentHash     *domain.ContentHash
	IdempotencyKey  string
	RequestID       string
	Auth            domain.AuthContext
}

type CompleteUploadSessionHandler struct {
	sessions  ports.SessionRepository
	parts     ports.SessionPartRepository
	blobs     ports.BlobRepository
	files     ports.FileRepository
	dedup     ports.DedupRepository
	usage     ports.TenantUsageRepository
	outbox    ports.OutboxPublisher
	txm       tx.Manager
	multipart ports.MultipartManager
	reader    ports.ObjectReader
	idgen     ids.Generator
	clock     clock.Clock
}

type materializedUpload struct {
	metadata      pkgstorage.ObjectMetadata
	providerParts []pkgstorage.UploadedPart
	canonicalHash *domain.ContentHash
	dedupDecision *domain.DedupDecision
}

type uploadSessionCompletedPayload struct {
	OccurredAt    time.Time `json:"occurredAt"`
	RequestID     string    `json:"requestId,omitempty"`
	TenantID      string    `json:"tenantId"`
	Producer      string    `json:"producer"`
	UploadSession string    `json:"uploadSessionId"`
	FileID        string    `json:"fileId"`
	BlobObjectID  string    `json:"blobObjectId"`
	HashAlgorithm string    `json:"hashAlgorithm,omitempty"`
	HashValue     string    `json:"hashValue,omitempty"`
	SizeBytes     int64     `json:"sizeBytes"`
}

func NewCompleteUploadSessionHandler(
	sessions ports.SessionRepository,
	parts ports.SessionPartRepository,
	blobs ports.BlobRepository,
	files ports.FileRepository,
	dedup ports.DedupRepository,
	usage ports.TenantUsageRepository,
	outbox ports.OutboxPublisher,
	txm tx.Manager,
	multipart ports.MultipartManager,
	reader ports.ObjectReader,
	idgen ids.Generator,
	clk clock.Clock,
) CompleteUploadSessionHandler {
	if txm == nil {
		txm = noopTxManager{}
	}
	if idgen == nil {
		idgen = ids.NewGenerator(nil, nil)
	}
	if clk == nil {
		clk = clock.SystemClock{}
	}

	return CompleteUploadSessionHandler{
		sessions:  sessions,
		parts:     parts,
		blobs:     blobs,
		files:     files,
		dedup:     dedup,
		usage:     usage,
		outbox:    outbox,
		txm:       txm,
		multipart: multipart,
		reader:    reader,
		idgen:     idgen,
		clock:     clk,
	}
}

func (h CompleteUploadSessionHandler) Handle(ctx context.Context, command CompleteUploadSessionCommand) (view.CompletedUploadSession, error) {
	if err := command.Auth.RequireFileWrite(); err != nil {
		return view.CompletedUploadSession{}, err
	}
	if strings.TrimSpace(command.UploadSessionID) == "" {
		return view.CompletedUploadSession{}, xerrors.New(xerrors.CodeInvalidArgument, "upload session id is required", xerrors.Details{
			"field": "uploadSessionId",
		})
	}

	session, err := h.sessions.GetByID(ctx, command.Auth.TenantID, command.UploadSessionID)
	if err != nil {
		return view.CompletedUploadSession{}, err
	}
	if err := command.Auth.EnsureSessionAccess(session); err != nil {
		return view.CompletedUploadSession{}, err
	}

	completionToken, err := h.completionToken(strings.TrimSpace(command.IdempotencyKey))
	if err != nil {
		return view.CompletedUploadSession{}, err
	}

	acquired, err := h.sessions.AcquireCompletion(ctx, ports.CompletionAcquireRequest{
		TenantID:        command.Auth.TenantID,
		UploadSessionID: command.UploadSessionID,
		CompletionToken: completionToken,
		StartedAt:       h.clock.Now(),
	})
	if err != nil {
		return view.CompletedUploadSession{}, err
	}

	ownedSession := session
	if acquired != nil && acquired.Session != nil {
		ownedSession = acquired.Session
	}
	if ownedSession == nil {
		return view.CompletedUploadSession{}, domain.ErrUploadSessionNotFound(command.UploadSessionID)
	}
	if acquired == nil {
		return view.CompletedUploadSession{}, xerrors.New(xerrors.CodeInternalError, "acquire completion returned nil result", nil)
	}

	switch acquired.Ownership {
	case domain.CompletionOwnershipAlreadyDone:
		return view.CompletedUploadSession{
			FileID:        ownedSession.FileID,
			UploadSession: view.FromSession(ownedSession, "", nil),
		}, nil
	case domain.CompletionOwnershipHeldByAnother:
		return view.CompletedUploadSession{}, xerrors.New(domain.CodeUploadCompleteInProgress, "upload completion is already in progress", xerrors.Details{
			"resourceType": "uploadSession",
			"resourceId":   ownedSession.ID,
		})
	case domain.CompletionOwnershipAcquired, domain.CompletionOwnershipHeldByCaller:
	default:
		return view.CompletedUploadSession{}, xerrors.New(xerrors.CodeInternalError, "unsupported completion ownership", xerrors.Details{
			"ownership": string(acquired.Ownership),
		})
	}
	if ownedSession.Status != domain.SessionStatusCompleting {
		startedAt := h.clock.Now()
		if acquired.Session != nil && !acquired.Session.UpdatedAt.IsZero() {
			startedAt = acquired.Session.UpdatedAt
		}
		if _, err := ownedSession.AcquireCompletion(completionToken, startedAt); err != nil && ownedSession.Status != domain.SessionStatusCompleting {
			return view.CompletedUploadSession{}, err
		}
	}

	materialized, err := h.materialize(ctx, ownedSession, command)
	if err != nil {
		return view.CompletedUploadSession{}, err
	}

	return h.commit(ctx, ownedSession, completionToken, materialized, command.RequestID)
}

func (h CompleteUploadSessionHandler) materialize(ctx context.Context, session *domain.Session, command CompleteUploadSessionCommand) (materializedUpload, error) {
	expectedHash, err := resolveExpectedContentHash(session.Hash, command.ContentHash)
	if err != nil {
		return materializedUpload{}, err
	}

	var providerParts []pkgstorage.UploadedPart
	if session.Mode == domain.SessionModeDirect {
		clientParts, err := normalizeUploadedParts(command.UploadedParts)
		if err != nil {
			return materializedUpload{}, err
		}
		if len(clientParts) == 0 {
			return materializedUpload{}, xerrors.New(domain.CodeUploadPartsMissing, "uploaded parts are incomplete", xerrors.Details{
				"expectedParts": session.TotalParts,
				"actualParts":   0,
			})
		}

		providerParts, err = h.multipart.ListUploadedParts(ctx, session.Object, session.ProviderUploadID)
		if err != nil {
			return materializedUpload{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "list uploaded parts", err, nil)
		}
		providerParts, err = normalizeUploadedParts(providerParts)
		if err != nil {
			return materializedUpload{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "normalize uploaded parts", err, nil)
		}
		if session.TotalParts > 0 && len(providerParts) < session.TotalParts {
			return materializedUpload{}, xerrors.New(domain.CodeUploadPartsMissing, "uploaded parts are incomplete", xerrors.Details{
				"expectedParts": session.TotalParts,
				"actualParts":   len(providerParts),
			})
		}
		if h.parts != nil {
			if err := h.parts.Replace(ctx, session.TenantID, session.ID, toSessionPartRecords(session.ID, providerParts, h.clock.Now())); err != nil {
				return materializedUpload{}, err
			}
		}
		if err := h.multipart.CompleteMultipartUpload(ctx, session.Object, session.ProviderUploadID, providerParts); err != nil {
			return materializedUpload{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "complete multipart upload", err, nil)
		}
	}

	metadata, err := h.reader.HeadObject(ctx, session.Object)
	if err != nil {
		return materializedUpload{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "head uploaded object", err, nil)
	}

	canonicalHash, err := resolveCanonicalHash(expectedHash, metadata.Checksum)
	if err != nil {
		return materializedUpload{}, err
	}

	var dedupDecision *domain.DedupDecision
	if canonicalHash != nil && h.dedup != nil {
		dedupDecision, err = h.dedup.LookupByHash(ctx, ports.DedupLookupKey{
			TenantID:   session.TenantID,
			BucketName: session.Object.BucketName,
			Hash:       *canonicalHash,
		})
		if err != nil {
			return materializedUpload{}, err
		}
		if dedupDecision != nil {
			if err := dedupDecision.Validate(); err != nil {
				return materializedUpload{}, err
			}
		}
	}

	return materializedUpload{
		metadata:      metadata,
		providerParts: providerParts,
		canonicalHash: canonicalHash,
		dedupDecision: dedupDecision,
	}, nil
}

func (h CompleteUploadSessionHandler) commit(
	ctx context.Context,
	session *domain.Session,
	completionToken string,
	materialized materializedUpload,
	requestID string,
) (view.CompletedUploadSession, error) {
	result := view.CompletedUploadSession{}

	err := h.txm.WithinTransaction(ctx, func(txCtx context.Context) error {
		ownedSession, err := h.sessions.ConfirmCompletionOwner(txCtx, session.TenantID, session.ID, completionToken)
		if err != nil {
			return err
		}
		if ownedSession == nil {
			return domain.ErrUploadSessionNotFound(session.ID)
		}
		if ownedSession.Status == domain.SessionStatusCompleted {
			result = view.CompletedUploadSession{
				FileID:        ownedSession.FileID,
				UploadSession: view.FromSession(ownedSession, "", nil),
			}
			return nil
		}

		blobID := ""
		if materialized.dedupDecision != nil && materialized.dedupDecision.Hit {
			blobID = materialized.dedupDecision.BlobID
		} else {
			blobID, err = h.idgen.New()
			if err != nil {
				return xerrors.Wrap(xerrors.CodeInternalError, "generate blob id", err, nil)
			}
		}

		fileID, err := h.idgen.New()
		if err != nil {
			return xerrors.Wrap(xerrors.CodeInternalError, "generate file id", err, nil)
		}

		sizeBytes := effectiveSizeBytes(materialized.metadata, ownedSession.SizeBytes)
		contentType := effectiveContentType(materialized.metadata, ownedSession.ContentType)
		hashValue := materialized.canonicalHash
		blobHash := domain.ContentHash{}
		if hashValue != nil {
			blobHash = *hashValue
		}

		if h.blobs != nil {
			if err := h.blobs.Upsert(txCtx, ports.BlobRecord{
				BlobID:          blobID,
				TenantID:        ownedSession.TenantID,
				StorageProvider: ownedSession.Object.Provider,
				BucketName:      ownedSession.Object.BucketName,
				ObjectKey:       ownedSession.Object.ObjectKey,
				SizeBytes:       sizeBytes,
				ContentType:     contentType,
				ETag:            strings.TrimSpace(materialized.metadata.ETag),
				Checksum:        strings.TrimSpace(materialized.metadata.Checksum),
				Hash:            blobHash,
			}); err != nil {
				return err
			}
			if err := h.blobs.AdjustReferenceCount(txCtx, blobID, 1); err != nil {
				return err
			}
		}

		if h.files != nil {
			if err := h.files.CreateFileAsset(txCtx, ports.FileAssetRecord{
				FileID:          fileID,
				TenantID:        ownedSession.TenantID,
				OwnerID:         ownedSession.OwnerID,
				BlobID:          blobID,
				FileName:        ownedSession.FileName,
				ContentType:     contentType,
				SizeBytes:       sizeBytes,
				AccessLevel:     ownedSession.AccessLevel,
				StorageProvider: ownedSession.Object.Provider,
				BucketName:      ownedSession.Object.BucketName,
				ObjectKey:       ownedSession.Object.ObjectKey,
				Hash:            cloneHash(hashValue),
			}); err != nil {
				return err
			}
		}

		if h.usage != nil {
			if err := h.usage.ApplyDelta(txCtx, ownedSession.TenantID, sizeBytes); err != nil {
				return err
			}
		}

		if len(materialized.providerParts) > 0 {
			ownedSession.CompletedParts = len(materialized.providerParts)
		} else {
			ownedSession.CompletedParts = ownedSession.TotalParts
		}
		if ownedSession.CompletedParts <= 0 {
			ownedSession.CompletedParts = 1
		}

		if err := ownedSession.MarkCompleted(fileID, h.clock.Now()); err != nil {
			return err
		}
		if err := h.sessions.Save(txCtx, ownedSession); err != nil {
			return err
		}

		if h.outbox != nil {
			payload, err := json.Marshal(uploadSessionCompletedPayload{
				OccurredAt:    h.clock.Now(),
				RequestID:     strings.TrimSpace(requestID),
				TenantID:      ownedSession.TenantID,
				Producer:      "upload-service",
				UploadSession: ownedSession.ID,
				FileID:        fileID,
				BlobObjectID:  blobID,
				HashAlgorithm: hashAlgorithm(hashValue),
				HashValue:     hashString(hashValue),
				SizeBytes:     sizeBytes,
			})
			if err != nil {
				return xerrors.Wrap(xerrors.CodeInternalError, "marshal upload completed payload", err, nil)
			}
			if err := h.outbox.Enqueue(txCtx, ports.OutboxMessage{
				EventType:   uploadSessionCompletedEventType,
				AggregateID: ownedSession.ID,
				Payload:     payload,
			}); err != nil {
				return err
			}
		}

		result = view.CompletedUploadSession{
			FileID:        fileID,
			UploadSession: view.FromSession(ownedSession, "", nil),
		}
		return nil
	})
	if err != nil {
		return view.CompletedUploadSession{}, err
	}

	return result, nil
}

func (h CompleteUploadSessionHandler) completionToken(idempotencyKey string) (string, error) {
	if idempotencyKey != "" {
		return idempotencyKey, nil
	}

	value, err := h.idgen.New()
	if err != nil {
		return "", xerrors.Wrap(xerrors.CodeInternalError, "generate completion token", err, nil)
	}
	return value, nil
}

func resolveExpectedContentHash(sessionHash *domain.ContentHash, commandHash *domain.ContentHash) (*domain.ContentHash, error) {
	normalizedCommand := normalizeHash(commandHash)
	if normalizedCommand != nil {
		if err := normalizedCommand.Validate(); err != nil {
			return nil, err
		}
	}

	normalizedSession := normalizeHash(sessionHash)
	if normalizedSession != nil && normalizedCommand != nil {
		if normalizedSession.Algorithm != normalizedCommand.Algorithm || normalizedSession.Value != normalizedCommand.Value {
			return nil, xerrors.New(domain.CodeUploadHashMismatch, "declared hash does not match upload session", xerrors.Details{
				"field": "contentHash",
			})
		}
	}
	if normalizedCommand != nil {
		return normalizedCommand, nil
	}
	return normalizedSession, nil
}

func resolveCanonicalHash(expected *domain.ContentHash, checksum string) (*domain.ContentHash, error) {
	verifiedChecksum := strings.ToLower(strings.TrimSpace(checksum))
	if expected != nil {
		if verifiedChecksum == "" {
			return nil, xerrors.New(domain.CodeUploadHashMismatch, "verified content hash is missing", xerrors.Details{
				"field": "contentHash",
			})
		}
		if verifiedChecksum != expected.Value {
			return nil, xerrors.New(domain.CodeUploadHashMismatch, "declared hash does not match verified hash", xerrors.Details{
				"field": "contentHash",
			})
		}
		confirmed := expected.Normalize()
		return &confirmed, nil
	}
	if verifiedChecksum == "" {
		return nil, nil
	}

	derived := domain.ContentHash{
		Algorithm: "SHA256",
		Value:     verifiedChecksum,
	}
	if err := derived.Validate(); err != nil {
		return nil, nil
	}
	return &derived, nil
}

func normalizeUploadedParts(parts []pkgstorage.UploadedPart) ([]pkgstorage.UploadedPart, error) {
	if len(parts) == 0 {
		return nil, nil
	}

	normalized := make([]pkgstorage.UploadedPart, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		if part.PartNumber < 1 {
			return nil, xerrors.New(xerrors.CodeInvalidArgument, "part number must be >= 1", xerrors.Details{
				"field":      "uploadedParts.partNumber",
				"partNumber": part.PartNumber,
			})
		}
		if _, ok := seen[part.PartNumber]; ok {
			return nil, xerrors.New(xerrors.CodeInvalidArgument, "duplicate part number", xerrors.Details{
				"field":      "uploadedParts.partNumber",
				"partNumber": part.PartNumber,
			})
		}
		seen[part.PartNumber] = struct{}{}
		normalized = append(normalized, pkgstorage.UploadedPart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(part.ETag),
			SizeBytes:  part.SizeBytes,
			Checksum:   strings.TrimSpace(part.Checksum),
		})
	}

	for i := 1; i < len(normalized); i++ {
		for j := i; j > 0 && normalized[j-1].PartNumber > normalized[j].PartNumber; j-- {
			normalized[j-1], normalized[j] = normalized[j], normalized[j-1]
		}
	}

	return normalized, nil
}

func toSessionPartRecords(uploadSessionID string, parts []pkgstorage.UploadedPart, uploadedAt time.Time) []ports.SessionPartRecord {
	records := make([]ports.SessionPartRecord, 0, len(parts))
	for _, part := range parts {
		records = append(records, ports.SessionPartRecord{
			UploadSessionID: uploadSessionID,
			PartNumber:      part.PartNumber,
			ETag:            strings.TrimSpace(part.ETag),
			PartSize:        part.SizeBytes,
			Checksum:        strings.TrimSpace(part.Checksum),
			UploadedAt:      uploadedAt.UTC(),
		})
	}
	return records
}

func effectiveSizeBytes(metadata pkgstorage.ObjectMetadata, fallback int64) int64 {
	if metadata.SizeBytes > 0 {
		return metadata.SizeBytes
	}
	return fallback
}

func effectiveContentType(metadata pkgstorage.ObjectMetadata, fallback string) string {
	if strings.TrimSpace(metadata.ContentType) != "" {
		return strings.TrimSpace(metadata.ContentType)
	}
	return strings.TrimSpace(fallback)
}

func cloneHash(hash *domain.ContentHash) *domain.ContentHash {
	if hash == nil {
		return nil
	}
	cloned := hash.Normalize()
	return &cloned
}

func hashAlgorithm(hash *domain.ContentHash) string {
	if hash == nil {
		return ""
	}
	return hash.Algorithm
}

func hashString(hash *domain.ContentHash) string {
	if hash == nil {
		return ""
	}
	return hash.Value
}

type noopTxManager struct{}

func (noopTxManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
