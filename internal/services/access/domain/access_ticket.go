package domain

import (
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

type DownloadDisposition string

const (
	DownloadDispositionInline     DownloadDisposition = "inline"
	DownloadDispositionAttachment DownloadDisposition = "attachment"
)

type AccessTicketClaims struct {
	FileID       string
	TenantID     string
	Subject      string
	SubjectType  string
	Disposition  DownloadDisposition
	ResponseName string
	ExpiresAt    time.Time
}

func NormalizeDisposition(disposition DownloadDisposition) (DownloadDisposition, error) {
	switch strings.TrimSpace(string(disposition)) {
	case "":
		return DownloadDispositionAttachment, nil
	case string(DownloadDispositionInline):
		return DownloadDispositionInline, nil
	case string(DownloadDispositionAttachment):
		return DownloadDispositionAttachment, nil
	default:
		return "", xerrors.New(xerrors.CodeInvalidArgument, "invalid download disposition", xerrors.Details{
			"field": "responseDisposition",
		})
	}
}

func (c AccessTicketClaims) Validate() error {
	if c.FileID == "" {
		return xerrors.New(xerrors.CodeInvalidArgument, "file id is required", xerrors.Details{"field": "fileId"})
	}
	if c.TenantID == "" {
		return xerrors.New(xerrors.CodeInvalidArgument, "tenant id is required", xerrors.Details{"field": "tenantId"})
	}
	if c.Subject == "" {
		return xerrors.New(xerrors.CodeInvalidArgument, "subject is required", xerrors.Details{"field": "subject"})
	}
	if c.ExpiresAt.IsZero() {
		return xerrors.New(xerrors.CodeInvalidArgument, "expires at is required", xerrors.Details{"field": "expiresAt"})
	}

	return nil
}
