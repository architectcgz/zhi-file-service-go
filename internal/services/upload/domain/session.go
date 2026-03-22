package domain

import (
	"fmt"
	"strings"
	"time"

	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type Session struct {
	ID                   string
	TenantID             string
	OwnerID              string
	FileName             string
	ContentType          string
	SizeBytes            int64
	AccessLevel          pkgstorage.AccessLevel
	Mode                 SessionMode
	Status               SessionStatus
	ChunkSizeBytes       int
	TotalParts           int
	CompletedParts       int
	Object               pkgstorage.ObjectRef
	ProviderUploadID     string
	FileID               string
	Hash                 *ContentHash
	CompletionToken      string
	CompletionStartedAt  *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CompletedAt          *time.Time
	AbortedAt            *time.Time
	FailureCode          string
	FailureMessage       string
	FailedAt             *time.Time
	ResumedFromSessionID string
	IdempotencyKey       string
	ExpiresAt            time.Time
}

type CreateSessionParams struct {
	ID                   string
	TenantID             string
	OwnerID              string
	FileName             string
	ContentType          string
	SizeBytes            int64
	AccessLevel          pkgstorage.AccessLevel
	Mode                 SessionMode
	ChunkSizeBytes       int
	TotalParts           int
	CompletedParts       int
	Object               pkgstorage.ObjectRef
	ProviderUploadID     string
	Hash                 *ContentHash
	Status               SessionStatus
	FileID               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CompletionToken      string
	CompletionStartedAt  *time.Time
	CompletedAt          *time.Time
	AbortedAt            *time.Time
	FailureCode          string
	FailureMessage       string
	FailedAt             *time.Time
	ResumedFromSessionID string
	IdempotencyKey       string
	ExpiresAt            time.Time
}

func NewSession(params CreateSessionParams) (*Session, error) {
	createdAt := params.CreatedAt.UTC()
	session := &Session{
		ID:                   strings.TrimSpace(params.ID),
		TenantID:             strings.TrimSpace(params.TenantID),
		OwnerID:              strings.TrimSpace(params.OwnerID),
		FileName:             strings.TrimSpace(params.FileName),
		ContentType:          strings.TrimSpace(params.ContentType),
		SizeBytes:            params.SizeBytes,
		AccessLevel:          params.AccessLevel,
		Mode:                 params.Mode,
		Status:               params.Status,
		ChunkSizeBytes:       params.ChunkSizeBytes,
		TotalParts:           params.TotalParts,
		CompletedParts:       params.CompletedParts,
		Object:               params.Object,
		ProviderUploadID:     strings.TrimSpace(params.ProviderUploadID),
		Hash:                 normalizeHash(params.Hash),
		FileID:               strings.TrimSpace(params.FileID),
		CompletionToken:      strings.TrimSpace(params.CompletionToken),
		CompletionStartedAt:  normalizeTimePtr(params.CompletionStartedAt),
		CreatedAt:            createdAt,
		UpdatedAt:            normalizeUpdatedAt(createdAt, params.UpdatedAt),
		CompletedAt:          normalizeTimePtr(params.CompletedAt),
		AbortedAt:            normalizeTimePtr(params.AbortedAt),
		FailureCode:          strings.TrimSpace(params.FailureCode),
		FailureMessage:       strings.TrimSpace(params.FailureMessage),
		FailedAt:             normalizeTimePtr(params.FailedAt),
		ResumedFromSessionID: strings.TrimSpace(params.ResumedFromSessionID),
		IdempotencyKey:       strings.TrimSpace(params.IdempotencyKey),
		ExpiresAt:            params.ExpiresAt.UTC(),
	}
	if session.Status == "" {
		session.Status = SessionStatusInitiated
	}
	if err := session.Validate(); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Session) Validate() error {
	if s == nil {
		return fmt.Errorf("session is required")
	}
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(s.TenantID) == "" {
		return fmt.Errorf("tenant id is required")
	}
	if strings.TrimSpace(s.OwnerID) == "" {
		return fmt.Errorf("owner id is required")
	}
	if strings.TrimSpace(s.FileName) == "" {
		return fmt.Errorf("file name is required")
	}
	if strings.TrimSpace(s.ContentType) == "" {
		return fmt.Errorf("content type is required")
	}
	if s.SizeBytes <= 0 {
		return fmt.Errorf("size bytes must be > 0")
	}
	if err := s.Mode.Validate(); err != nil {
		return err
	}
	if err := s.Status.Validate(); err != nil {
		return err
	}
	if err := validateAccessLevel(s.AccessLevel); err != nil {
		return err
	}
	if err := s.Object.Validate(); err != nil {
		return err
	}
	if s.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	if s.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}
	if s.UpdatedAt.Before(s.CreatedAt) {
		return fmt.Errorf("updated at must be >= created at")
	}
	if s.ExpiresAt.IsZero() {
		return fmt.Errorf("expires at is required")
	}
	if !s.ExpiresAt.After(s.CreatedAt) {
		return fmt.Errorf("expires at must be after created at")
	}
	if s.ChunkSizeBytes < 0 {
		return fmt.Errorf("chunk size bytes must be >= 0")
	}

	if s.Mode.RequiresContentHash() && s.Hash == nil {
		return errUploadHashRequired(s.Mode)
	}
	if s.Hash != nil {
		if err := s.Hash.Validate(); err != nil {
			return err
		}
	}

	if s.Mode.RequiresProviderUploadID() {
		if s.ProviderUploadID == "" {
			return fmt.Errorf("provider upload id is required for %s", s.Mode)
		}
		if s.TotalParts < 1 {
			return fmt.Errorf("total parts must be >= 1 for %s", s.Mode)
		}
	} else {
		if s.ProviderUploadID != "" {
			return fmt.Errorf("provider upload id must be empty for %s", s.Mode)
		}
		if s.TotalParts == 0 {
			s.TotalParts = 1
		}
		if s.TotalParts != 1 {
			return fmt.Errorf("total parts must be 1 for %s", s.Mode)
		}
	}

	if s.CompletedParts < 0 {
		return fmt.Errorf("completed parts must be >= 0")
	}
	if s.TotalParts > 0 && s.CompletedParts > s.TotalParts {
		return fmt.Errorf("completed parts must be <= total parts")
	}

	if s.Status == SessionStatusCompleting {
		if err := validateCompletionOwnership(s.CompletionToken, s.CompletionStartedAt); err != nil {
			return err
		}
	}

	switch s.Status {
	case SessionStatusCompleted:
		if s.FileID == "" {
			return fmt.Errorf("file id is required when session is completed")
		}
		if s.CompletedAt == nil || s.CompletedAt.IsZero() {
			return fmt.Errorf("completed at is required when session is completed")
		}
	case SessionStatusAborted:
		if s.AbortedAt == nil || s.AbortedAt.IsZero() {
			return fmt.Errorf("aborted at is required when session is aborted")
		}
	case SessionStatusFailed:
		if s.FailedAt == nil || s.FailedAt.IsZero() {
			return fmt.Errorf("failed at is required when session is failed")
		}
		if s.FailureCode == "" || s.FailureMessage == "" {
			return fmt.Errorf("failure code and message are required when session is failed")
		}
	}

	return nil
}

func (s *Session) MarkUploading(completedParts int) error {
	switch s.Status {
	case SessionStatusInitiated, SessionStatusUploading:
	default:
		return errUploadSessionStateConflict(s.Status, SessionStatusInitiated, SessionStatusUploading)
	}

	s.Status = SessionStatusUploading
	if completedParts > s.CompletedParts {
		s.CompletedParts = completedParts
	}
	return nil
}

func (s *Session) AcquireCompletion(token string, startedAt time.Time) (CompletionOwnership, error) {
	if err := validateCompletionOwnership(token, &startedAt); err != nil {
		return "", err
	}

	switch s.Status {
	case SessionStatusInitiated, SessionStatusUploading:
		value := startedAt.UTC()
		s.Status = SessionStatusCompleting
		s.CompletionToken = strings.TrimSpace(token)
		s.CompletionStartedAt = &value
		s.UpdatedAt = value
		return CompletionOwnershipAcquired, nil
	case SessionStatusCompleting:
		if err := validateCompletionOwnership(s.CompletionToken, s.CompletionStartedAt); err != nil {
			return "", err
		}
		if s.CompletionToken == strings.TrimSpace(token) {
			return CompletionOwnershipHeldByCaller, nil
		}
		return CompletionOwnershipHeldByAnother, nil
	case SessionStatusCompleted:
		return CompletionOwnershipAlreadyDone, nil
	default:
		return "", errUploadSessionStateConflict(s.Status, SessionStatusInitiated, SessionStatusUploading, SessionStatusCompleting, SessionStatusCompleted)
	}
}

func (s *Session) MarkCompleted(fileID string, completedAt time.Time) error {
	if s.Status == SessionStatusCompleted {
		if s.FileID != "" && s.FileID != strings.TrimSpace(fileID) {
			return errUploadSessionStateConflict(s.Status, SessionStatusCompleted)
		}
		return nil
	}
	if s.Status != SessionStatusCompleting {
		return errUploadSessionStateConflict(s.Status, SessionStatusCompleting, SessionStatusCompleted)
	}

	s.Status = SessionStatusCompleted
	s.FileID = strings.TrimSpace(fileID)
	value := completedAt.UTC()
	s.CompletedAt = &value
	s.UpdatedAt = value
	return s.Validate()
}

func (s *Session) MarkFailed(code string, message string, failedAt time.Time) error {
	if s.Status == SessionStatusFailed {
		return nil
	}
	if s.Status != SessionStatusCompleting {
		return errUploadSessionStateConflict(s.Status, SessionStatusCompleting, SessionStatusFailed)
	}

	value := failedAt.UTC()
	s.Status = SessionStatusFailed
	s.FailureCode = strings.TrimSpace(code)
	s.FailureMessage = strings.TrimSpace(message)
	s.FailedAt = &value
	s.UpdatedAt = value
	return s.Validate()
}

func (s *Session) Abort(abortedAt time.Time) error {
	switch s.Status {
	case SessionStatusAborted, SessionStatusExpired, SessionStatusFailed:
		return nil
	case SessionStatusInitiated, SessionStatusUploading:
		s.Status = SessionStatusAborted
		value := abortedAt.UTC()
		s.AbortedAt = &value
		s.UpdatedAt = value
		return nil
	case SessionStatusCompleted:
		return errUploadSessionStateConflict(s.Status, SessionStatusInitiated, SessionStatusUploading)
	default:
		return errUploadSessionStateConflict(s.Status, SessionStatusInitiated, SessionStatusUploading, SessionStatusAborted, SessionStatusExpired, SessionStatusFailed)
	}
}

func normalizeHash(hash *ContentHash) *ContentHash {
	if hash == nil {
		return nil
	}
	normalized := hash.Normalize()
	return &normalized
}

func validateAccessLevel(level pkgstorage.AccessLevel) error {
	switch level {
	case pkgstorage.AccessLevelPublic, pkgstorage.AccessLevelPrivate:
		return nil
	default:
		return fmt.Errorf("access level is invalid: %s", level)
	}
}

func normalizeUpdatedAt(createdAt time.Time, updatedAt time.Time) time.Time {
	if updatedAt.IsZero() {
		return createdAt.UTC()
	}
	return updatedAt.UTC()
}

func normalizeTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func validateCompletionOwnership(token string, startedAt *time.Time) error {
	if strings.TrimSpace(token) == "" {
		return errUploadCompletionOwnershipInvalid("completion token is required")
	}
	if startedAt == nil || startedAt.IsZero() {
		return errUploadCompletionOwnershipInvalid("completion started at is required")
	}
	return nil
}
