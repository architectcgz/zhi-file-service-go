package commands

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type CreateUploadSessionCommand struct {
	FileName       string
	ContentType    string
	SizeBytes      int64
	ContentHash    *domain.ContentHash
	AccessLevel    pkgstorage.AccessLevel
	UploadMode     domain.SessionMode
	TotalParts     int
	Metadata       map[string]string
	IdempotencyKey string
	Auth           domain.AuthContext
}

type CreateUploadSessionConfig struct {
	SessionTTL   time.Duration
	PresignTTL   time.Duration
	AllowedModes []domain.SessionMode
}

type CreateUploadSessionHandler struct {
	sessions     ports.SessionRepository
	policies     ports.TenantPolicyReader
	buckets      ports.BucketResolver
	multipart    ports.MultipartManager
	presign      ports.PresignManager
	idgen        ids.Generator
	clock        clock.Clock
	sessionTTL   time.Duration
	presignTTL   time.Duration
	allowedModes map[domain.SessionMode]struct{}
}

func NewCreateUploadSessionHandler(
	sessions ports.SessionRepository,
	policies ports.TenantPolicyReader,
	buckets ports.BucketResolver,
	multipart ports.MultipartManager,
	presign ports.PresignManager,
	idgen ids.Generator,
	clk clock.Clock,
	cfg CreateUploadSessionConfig,
) CreateUploadSessionHandler {
	if idgen == nil {
		idgen = ids.NewGenerator(nil, nil)
	}
	if clk == nil {
		clk = clock.SystemClock{}
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 30 * time.Minute
	}
	if cfg.PresignTTL <= 0 {
		cfg.PresignTTL = 15 * time.Minute
	}

	return CreateUploadSessionHandler{
		sessions:     sessions,
		policies:     policies,
		buckets:      buckets,
		multipart:    multipart,
		presign:      presign,
		idgen:        idgen,
		clock:        clk,
		sessionTTL:   cfg.SessionTTL,
		presignTTL:   cfg.PresignTTL,
		allowedModes: allowedModeSet(cfg.AllowedModes),
	}
}

func (h CreateUploadSessionHandler) Handle(ctx context.Context, command CreateUploadSessionCommand) (view.UploadSession, error) {
	if err := command.Auth.RequireFileWrite(); err != nil {
		return view.UploadSession{}, err
	}

	fileName := strings.TrimSpace(command.FileName)
	if fileName == "" {
		return view.UploadSession{}, xerrors.New(xerrors.CodeInvalidArgument, "file name is required", xerrors.Details{
			"field": "fileName",
		})
	}
	contentType := strings.TrimSpace(command.ContentType)
	if contentType == "" {
		return view.UploadSession{}, xerrors.New(xerrors.CodeInvalidArgument, "content type is required", xerrors.Details{
			"field": "contentType",
		})
	}
	if command.SizeBytes <= 0 {
		return view.UploadSession{}, xerrors.New(xerrors.CodeInvalidArgument, "size bytes must be > 0", xerrors.Details{
			"field": "sizeBytes",
		})
	}

	mode := command.UploadMode
	if err := mode.Validate(); err != nil {
		return view.UploadSession{}, err
	}
	if !h.isModeAllowed(mode) {
		return view.UploadSession{}, newUploadModeRejected(mode, "upload_mode_disabled")
	}

	accessLevel := normalizeAccessLevel(command.AccessLevel)
	hash := normalizeHash(command.ContentHash)
	if hash != nil {
		if err := hash.Validate(); err != nil {
			return view.UploadSession{}, err
		}
	}

	policy, err := h.policies.ReadUploadPolicy(ctx, command.Auth.TenantID)
	if err != nil {
		return view.UploadSession{}, err
	}
	if err := validateUploadPolicy(policy, mode, contentType, command.SizeBytes); err != nil {
		return view.UploadSession{}, err
	}

	reusable, err := h.sessions.FindReusable(ctx, ports.ReusableSessionQuery{
		TenantID:    command.Auth.TenantID,
		OwnerID:     command.Auth.SubjectID,
		Mode:        mode,
		AccessLevel: accessLevel,
		SizeBytes:   command.SizeBytes,
		Hash:        hash,
	})
	if err != nil {
		return view.UploadSession{}, err
	}
	if reusable != nil {
		if err := command.Auth.EnsureSessionAccess(reusable); err != nil {
			return view.UploadSession{}, err
		}

		putURL, putHeaders, err := h.issuePutURL(ctx, reusable)
		if err != nil {
			return view.UploadSession{}, err
		}

		return view.FromSession(reusable, putURL, putHeaders), nil
	}

	sessionID, err := h.idgen.New()
	if err != nil {
		return view.UploadSession{}, xerrors.Wrap(xerrors.CodeInternalError, "generate upload session id", err, nil)
	}
	bucket, err := h.buckets.Resolve(accessLevel)
	if err != nil {
		return view.UploadSession{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "resolve upload bucket", err, nil)
	}
	objectRef := pkgstorage.ObjectRef{
		Provider:   bucket.Provider,
		BucketName: bucket.BucketName,
		ObjectKey:  planObjectKey(command.Auth.TenantID, sessionID, fileName),
	}

	providerUploadID := ""
	putURL := ""
	var putHeaders map[string]string
	switch mode {
	case domain.SessionModeDirect:
		providerUploadID, err = h.multipart.CreateMultipartUpload(ctx, objectRef, contentType)
		if err != nil {
			return view.UploadSession{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "create multipart upload", err, nil)
		}
	case domain.SessionModePresignedSingle:
		putURL, putHeaders, err = h.presign.PresignPutObject(ctx, objectRef, contentType, h.presignTTL)
		if err != nil {
			return view.UploadSession{}, xerrors.Wrap(xerrors.CodeServiceUnavailable, "presign single upload", err, nil)
		}
	}

	now := h.clock.Now()
	session, err := domain.NewSession(domain.CreateSessionParams{
		ID:               sessionID,
		TenantID:         command.Auth.TenantID,
		OwnerID:          command.Auth.SubjectID,
		FileName:         fileName,
		ContentType:      contentType,
		SizeBytes:        command.SizeBytes,
		AccessLevel:      accessLevel,
		Mode:             mode,
		TotalParts:       normalizeTotalParts(mode, command.TotalParts),
		Object:           objectRef,
		ProviderUploadID: providerUploadID,
		Hash:             hash,
		CreatedAt:        now,
		UpdatedAt:        now,
		IdempotencyKey:   strings.TrimSpace(command.IdempotencyKey),
		ExpiresAt:        now.Add(h.sessionTTL),
	})
	if err != nil {
		return view.UploadSession{}, err
	}

	if err := h.sessions.Create(ctx, session); err != nil {
		return view.UploadSession{}, err
	}

	return view.FromSession(session, putURL, putHeaders), nil
}

func (h CreateUploadSessionHandler) issuePutURL(ctx context.Context, session *domain.Session) (string, map[string]string, error) {
	if session == nil || session.Mode != domain.SessionModePresignedSingle {
		return "", nil, nil
	}

	putURL, headers, err := h.presign.PresignPutObject(ctx, session.Object, session.ContentType, h.presignTTL)
	if err != nil {
		return "", nil, xerrors.Wrap(xerrors.CodeServiceUnavailable, "presign single upload", err, nil)
	}

	return putURL, headers, nil
}

func (h CreateUploadSessionHandler) isModeAllowed(mode domain.SessionMode) bool {
	if len(h.allowedModes) == 0 {
		return true
	}

	_, ok := h.allowedModes[mode]
	return ok
}

func normalizeHash(hash *domain.ContentHash) *domain.ContentHash {
	if hash == nil {
		return nil
	}

	normalized := hash.Normalize()
	return &normalized
}

func normalizeAccessLevel(accessLevel pkgstorage.AccessLevel) pkgstorage.AccessLevel {
	if accessLevel == "" {
		return pkgstorage.AccessLevelPrivate
	}

	return accessLevel
}

func normalizeTotalParts(mode domain.SessionMode, totalParts int) int {
	if totalParts > 0 {
		return totalParts
	}

	return mode.DefaultTotalParts()
}

func allowedModeSet(modes []domain.SessionMode) map[domain.SessionMode]struct{} {
	if len(modes) == 0 {
		return nil
	}

	allowed := make(map[domain.SessionMode]struct{}, len(modes))
	for _, mode := range modes {
		allowed[mode] = struct{}{}
	}

	return allowed
}

func validateUploadPolicy(policy ports.TenantUploadPolicy, mode domain.SessionMode, contentType string, sizeBytes int64) error {
	if policy.MaxFileSize > 0 && sizeBytes > policy.MaxFileSize {
		return newTenantQuotaExceeded(policy.MaxFileSize)
	}
	if len(policy.AllowedMimeTypes) > 0 && !containsFold(policy.AllowedMimeTypes, contentType) {
		return newMimeTypeNotAllowed(contentType)
	}

	switch mode {
	case domain.SessionModeInline:
		if !policy.AllowInlineUpload {
			return newUploadModeRejected(mode, "inline_upload_disabled")
		}
		if policy.MaxInlineSize > 0 && sizeBytes > policy.MaxInlineSize {
			return newUploadModeRejected(mode, "max_inline_size_exceeded")
		}
	case domain.SessionModeDirect:
		if !policy.AllowMultipart {
			return newUploadModeRejected(mode, "multipart_upload_disabled")
		}
	}

	return nil
}

func containsFold(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}

	return false
}

func planObjectKey(tenantID string, sessionID string, fileName string) string {
	return path.Join(
		strings.TrimSpace(tenantID),
		"uploads",
		strings.TrimSpace(sessionID),
		sanitizeFileName(fileName),
	)
}

func sanitizeFileName(fileName string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_")
	fileName = strings.TrimSpace(replacer.Replace(fileName))
	fileName = strings.Trim(fileName, "._")
	if fileName == "" {
		return "file"
	}

	return fileName
}

func newUploadModeRejected(mode domain.SessionMode, reason string) error {
	return xerrors.New(domain.CodeUploadModeInvalid, "upload mode is not allowed by current policy", xerrors.Details{
		"uploadMode": string(mode),
		"reason":     reason,
	})
}

func newTenantQuotaExceeded(limit int64) error {
	return xerrors.New(domain.CodeTenantQuotaExceeded, "tenant quota exceeded", xerrors.Details{
		"limit": limit,
	})
}

func newMimeTypeNotAllowed(contentType string) error {
	return xerrors.New(domain.CodeMimeTypeNotAllowed, "mime type is not allowed", xerrors.Details{
		"contentType": contentType,
	})
}
