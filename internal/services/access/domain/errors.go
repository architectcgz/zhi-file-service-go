package domain

import "github.com/architectcgz/zhi-file-service-go/pkg/xerrors"

const (
	CodeFileNotFound        xerrors.Code = "FILE_NOT_FOUND"
	CodeFileAccessDenied    xerrors.Code = "FILE_ACCESS_DENIED"
	CodeAccessTicketInvalid xerrors.Code = "ACCESS_TICKET_INVALID"
	CodeAccessTicketExpired xerrors.Code = "ACCESS_TICKET_EXPIRED"
	CodeDownloadNotAllowed  xerrors.Code = "DOWNLOAD_NOT_ALLOWED"
	CodeTenantScopeDenied   xerrors.Code = "TENANT_SCOPE_DENIED"
)

func ErrFileNotFound(fileID string) error {
	return xerrors.New(CodeFileNotFound, "file not found", xerrors.Details{
		"resourceType": "file",
		"resourceId":   fileID,
	})
}

func ErrFileAccessDenied(fileID string) error {
	return xerrors.New(CodeFileAccessDenied, "file access denied", xerrors.Details{
		"resourceType": "file",
		"resourceId":   fileID,
	})
}

func ErrAccessTicketInvalid(ticket string) error {
	return xerrors.New(CodeAccessTicketInvalid, "access ticket is invalid", xerrors.Details{
		"resourceType": "accessTicket",
		"resourceId":   ticket,
	})
}

func ErrAccessTicketExpired(ticket string) error {
	return xerrors.New(CodeAccessTicketExpired, "access ticket is expired", xerrors.Details{
		"resourceType": "accessTicket",
		"resourceId":   ticket,
	})
}

func ErrDownloadNotAllowed(reason string) error {
	return xerrors.New(CodeDownloadNotAllowed, "file download is not allowed by current policy", xerrors.Details{
		"reason": reason,
	})
}

func ErrTenantScopeDenied(fileID string) error {
	return xerrors.New(CodeTenantScopeDenied, "tenant scope denied", xerrors.Details{
		"resourceType": "file",
		"resourceId":   fileID,
	})
}
