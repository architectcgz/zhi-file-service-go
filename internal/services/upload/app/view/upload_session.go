package view

import (
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type UploadSession struct {
	UploadSessionID string
	TenantID        string
	UploadMode      domain.SessionMode
	Status          domain.SessionStatus
	FileName        string
	ContentType     string
	SizeBytes       int64
	AccessLevel     pkgstorage.AccessLevel
	TotalParts      int
	UploadedParts   int
	PutURL          string
	PutHeaders      map[string]string
	FileID          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
}

type UploadedPart struct {
	PartNumber int
	ETag       string
	SizeBytes  int64
}

func FromSession(session *domain.Session, putURL string, putHeaders map[string]string) UploadSession {
	if session == nil {
		return UploadSession{}
	}

	return UploadSession{
		UploadSessionID: session.ID,
		TenantID:        session.TenantID,
		UploadMode:      session.Mode,
		Status:          session.Status,
		FileName:        session.FileName,
		ContentType:     session.ContentType,
		SizeBytes:       session.SizeBytes,
		AccessLevel:     session.AccessLevel,
		TotalParts:      session.TotalParts,
		UploadedParts:   session.CompletedParts,
		PutURL:          putURL,
		PutHeaders:      cloneHeaders(putHeaders),
		FileID:          session.FileID,
		CreatedAt:       session.CreatedAt,
		UpdatedAt:       session.UpdatedAt,
		CompletedAt:     session.CompletedAt,
	}
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}

	return cloned
}
