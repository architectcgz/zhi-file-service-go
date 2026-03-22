package queries

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
)

type ListUploadedPartsQuery struct {
	UploadSessionID string
	Auth            domain.AuthContext
}

type ListUploadedPartsResult struct {
	Parts []view.UploadedPart
}

type ListUploadedPartsHandler struct {
	sessions  ports.SessionRepository
	parts     ports.SessionPartRepository
	multipart ports.MultipartManager
	clock     clock.Clock
}

func NewListUploadedPartsHandler(
	sessions ports.SessionRepository,
	parts ports.SessionPartRepository,
	multipart ports.MultipartManager,
	clk clock.Clock,
) ListUploadedPartsHandler {
	if clk == nil {
		clk = clock.SystemClock{}
	}

	return ListUploadedPartsHandler{
		sessions:  sessions,
		parts:     parts,
		multipart: multipart,
		clock:     clk,
	}
}

func (h ListUploadedPartsHandler) Handle(ctx context.Context, query ListUploadedPartsQuery) (ListUploadedPartsResult, error) {
	if err := query.Auth.RequireFileWrite(); err != nil {
		return ListUploadedPartsResult{}, err
	}

	session, err := h.sessions.GetByID(ctx, query.Auth.TenantID, query.UploadSessionID)
	if err != nil {
		return ListUploadedPartsResult{}, err
	}
	if err := query.Auth.EnsureSessionAccess(session); err != nil {
		return ListUploadedPartsResult{}, err
	}

	if session.Mode != domain.SessionModeDirect {
		records, err := h.parts.ListBySessionID(ctx, session.TenantID, session.ID)
		if err != nil {
			return ListUploadedPartsResult{}, err
		}

		return ListUploadedPartsResult{Parts: mapPartRecords(records)}, nil
	}

	providerParts, err := h.multipart.ListUploadedParts(ctx, session.Object, session.ProviderUploadID)
	if err != nil {
		return ListUploadedPartsResult{}, err
	}

	records := make([]ports.SessionPartRecord, 0, len(providerParts))
	now := h.clock.Now()
	for _, part := range providerParts {
		records = append(records, ports.SessionPartRecord{
			UploadSessionID: session.ID,
			PartNumber:      part.PartNumber,
			ETag:            part.ETag,
			PartSize:        part.SizeBytes,
			Checksum:        part.Checksum,
			UploadedAt:      now,
		})
	}
	if err := h.parts.Replace(ctx, session.TenantID, session.ID, records); err != nil {
		return ListUploadedPartsResult{}, err
	}
	if len(providerParts) > 0 {
		if err := session.MarkUploading(len(providerParts)); err == nil {
			session.UpdatedAt = now
			if err := h.sessions.Save(ctx, session); err != nil {
				return ListUploadedPartsResult{}, err
			}
		}
	}

	return ListUploadedPartsResult{Parts: mapUploadedParts(providerParts)}, nil
}

func mapPartRecords(records []ports.SessionPartRecord) []view.UploadedPart {
	parts := make([]view.UploadedPart, 0, len(records))
	for _, record := range records {
		parts = append(parts, view.UploadedPart{
			PartNumber: record.PartNumber,
			ETag:       record.ETag,
			SizeBytes:  record.PartSize,
		})
	}

	return parts
}

func mapUploadedParts(records []pkgstorage.UploadedPart) []view.UploadedPart {
	parts := make([]view.UploadedPart, 0, len(records))
	for _, record := range records {
		parts = append(parts, view.UploadedPart{
			PartNumber: record.PartNumber,
			ETag:       record.ETag,
			SizeBytes:  record.SizeBytes,
		})
	}

	return parts
}
