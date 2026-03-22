package domain

type TenantPolicy struct {
	TenantID                   string
	DownloadDisabled           bool
	InlinePreviewDisabled      bool
	AttachmentDownloadDisabled bool
}

func (p TenantPolicy) EnsureAllowed(disposition DownloadDisposition) error {
	if p.DownloadDisabled {
		return ErrDownloadNotAllowed("download_disabled")
	}

	switch disposition {
	case DownloadDispositionInline:
		if p.InlinePreviewDisabled {
			return ErrDownloadNotAllowed("inline_preview_disabled")
		}
	case DownloadDispositionAttachment:
		if p.AttachmentDownloadDisabled {
			return ErrDownloadNotAllowed("attachment_download_disabled")
		}
	}

	return nil
}
