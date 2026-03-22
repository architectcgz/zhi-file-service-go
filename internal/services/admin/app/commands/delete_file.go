package commands

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/app/view"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/architectcgz/zhi-file-service-go/pkg/ids"
)

const fileDeleteRequestedEventType = "file.asset.delete_requested.v1"

type DeleteFileCommand struct {
	FileID         string
	Reason         string
	IdempotencyKey string
	Auth           domain.AdminContext
}

type DeleteFileHandler struct {
	files  ports.AdminFileRepository
	audits ports.AuditLogRepository
	outbox ports.OutboxPublisher
	tx     ports.TxManager
	idgen  ids.Generator
	clock  clock.Clock
}

func NewDeleteFileHandler(
	files ports.AdminFileRepository,
	audits ports.AuditLogRepository,
	outbox ports.OutboxPublisher,
	tx ports.TxManager,
	idgen ids.Generator,
	clk clock.Clock,
) DeleteFileHandler {
	return DeleteFileHandler{
		files:  files,
		audits: audits,
		outbox: outbox,
		tx:     defaultTxManager(tx),
		idgen:  defaultIDGenerator(idgen),
		clock:  defaultClock(clk),
	}
}

func (h DeleteFileHandler) Handle(ctx context.Context, command DeleteFileCommand) (view.DeleteFileResult, error) {
	fileID, err := requiredField(command.FileID, "fileId")
	if err != nil {
		return view.DeleteFileResult{}, err
	}

	current, err := h.files.GetByID(ctx, fileID)
	if err != nil {
		return view.DeleteFileResult{}, err
	}
	if current == nil {
		return view.DeleteFileResult{}, domain.ErrFileNotFound(fileID)
	}
	if err := newGuard().EnsureDeleteFile(command.Auth, current.TenantID, command.Reason); err != nil {
		return view.DeleteFileResult{}, err
	}

	var deleted *ports.DeleteFileRecord
	if err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		now := h.clock.Now()
		deleted, err = h.files.MarkDeleted(txCtx, fileID, now)
		if err != nil {
			return err
		}
		if deleted == nil {
			return domain.ErrFileNotFound(fileID)
		}
		if deleted.AlreadyDeleted {
			return nil
		}

		record, err := newAuditRecord(h.idgen, h.clock, command.Auth, deleted.File.TenantID, actionFileDelete, "file", deleted.File.FileID, map[string]any{
			"reason":         command.Reason,
			"idempotencyKey": optionalField(command.IdempotencyKey),
			"blobId":         deleted.File.BlobID,
		})
		if err != nil {
			return err
		}
		if err := h.audits.Append(txCtx, record); err != nil {
			return err
		}

		return h.outbox.Publish(txCtx, ports.OutboxEvent{
			EventType:     fileDeleteRequestedEventType,
			AggregateType: "file_asset",
			AggregateID:   deleted.File.FileID,
			OccurredAt:    now,
			RequestID:     command.Auth.RequestID,
			TenantID:      deleted.File.TenantID,
			Payload: map[string]any{
				"occurredAt": now.UTC().Format(time.RFC3339),
				"requestId":  command.Auth.RequestID,
				"tenantId":   deleted.File.TenantID,
				"producer":   "admin-service",
				"fileId":     deleted.File.FileID,
				"blobObjectId": deleted.File.BlobID,
				"deletedBy":  command.Auth.AdminID,
				"reason":     command.Reason,
			},
		})
	}); err != nil {
		return view.DeleteFileResult{}, err
	}

	return view.FromDeleteFileRecord(deleted), nil
}
