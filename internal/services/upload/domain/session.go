package domain

import (
	"fmt"
	"strings"
	"time"

	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type Session struct {
	ID                  string
	TenantID            string
	FileName            string
	ContentType         string
	SizeBytes           int64
	AccessLevel         pkgstorage.AccessLevel
	Mode                SessionMode
	Status              SessionStatus
	TotalParts          int
	UploadedParts       int
	Object              pkgstorage.ObjectRef
	ProviderUploadID    string
	FileID              string
	Hash                *ContentHash
	CompletionToken     string
	CompletionStartedAt *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	CompletedAt         *time.Time
	AbortedAt           *time.Time
	ExpiresAt           time.Time
}

type CreateSessionParams struct {
	ID               string
	TenantID         string
	FileName         string
	ContentType      string
	SizeBytes        int64
	AccessLevel      pkgstorage.AccessLevel
	Mode             SessionMode
	TotalParts       int
	Object           pkgstorage.ObjectRef
	ProviderUploadID string
	Hash             *ContentHash
	Status           SessionStatus
	FileID           string
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

func NewSession(params CreateSessionParams) (*Session, error) {
	session := &Session{
		ID:               strings.TrimSpace(params.ID),
		TenantID:         strings.TrimSpace(params.TenantID),
		FileName:         strings.TrimSpace(params.FileName),
		ContentType:      strings.TrimSpace(params.ContentType),
		SizeBytes:        params.SizeBytes,
		AccessLevel:      params.AccessLevel,
		Mode:             params.Mode,
		Status:           params.Status,
		TotalParts:       params.TotalParts,
		Object:           params.Object,
		ProviderUploadID: strings.TrimSpace(params.ProviderUploadID),
		Hash:             normalizeHash(params.Hash),
		FileID:           strings.TrimSpace(params.FileID),
		CreatedAt:        params.CreatedAt.UTC(),
		UpdatedAt:        params.CreatedAt.UTC(),
		ExpiresAt:        params.ExpiresAt.UTC(),
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
	if s.ExpiresAt.IsZero() {
		return fmt.Errorf("expires at is required")
	}
	if !s.ExpiresAt.After(s.CreatedAt) {
		return fmt.Errorf("expires at must be after created at")
	}

	if s.Mode.RequiresContentHash() {
		if s.Hash == nil {
			return errUploadHashRequired(s.Mode)
		}
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

	if s.Status == SessionStatusCompleted && s.FileID == "" {
		return fmt.Errorf("file id is required when session is completed")
	}

	return nil
}

func (s *Session) MarkUploading(uploadedParts int) error {
	switch s.Status {
	case SessionStatusInitiated, SessionStatusUploading:
	default:
		return errUploadSessionStateConflict(s.Status, SessionStatusInitiated, SessionStatusUploading)
	}

	s.Status = SessionStatusUploading
	if uploadedParts > s.UploadedParts {
		s.UploadedParts = uploadedParts
	}
	return nil
}

func (s *Session) AcquireCompletion(token string, startedAt time.Time) error {
	switch s.Status {
	case SessionStatusInitiated, SessionStatusUploading:
		s.Status = SessionStatusCompleting
		s.CompletionToken = strings.TrimSpace(token)
		value := startedAt.UTC()
		s.CompletionStartedAt = &value
		return nil
	case SessionStatusCompleting, SessionStatusCompleted:
		return nil
	default:
		return errUploadSessionStateConflict(s.Status, SessionStatusInitiated, SessionStatusUploading, SessionStatusCompleting, SessionStatusCompleted)
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
