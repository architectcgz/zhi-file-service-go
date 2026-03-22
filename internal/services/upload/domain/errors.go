package domain

import "github.com/architectcgz/zhi-file-service-go/pkg/xerrors"

const (
	CodeUploadSessionNotFound             xerrors.Code = "UPLOAD_SESSION_NOT_FOUND"
	CodeUploadModeInvalid                xerrors.Code = "UPLOAD_MODE_INVALID"
	CodeUploadHashRequired               xerrors.Code = "UPLOAD_HASH_REQUIRED"
	CodeUploadHashInvalid                xerrors.Code = "UPLOAD_HASH_INVALID"
	CodeUploadHashUnsupported            xerrors.Code = "UPLOAD_HASH_UNSUPPORTED"
	CodeUploadCompletionOwnershipInvalid xerrors.Code = "UPLOAD_COMPLETION_OWNERSHIP_INVALID"
	CodeUploadSessionStateConflict       xerrors.Code = "UPLOAD_SESSION_STATE_CONFLICT"
	CodeTenantQuotaExceeded              xerrors.Code = "TENANT_QUOTA_EXCEEDED"
	CodeMimeTypeNotAllowed               xerrors.Code = "MIME_TYPE_NOT_ALLOWED"
)

func ErrUploadSessionNotFound(uploadSessionID string) error {
	return xerrors.New(CodeUploadSessionNotFound, "upload session not found", xerrors.Details{
		"resourceType": "uploadSession",
		"resourceId":   uploadSessionID,
	})
}

func errUploadModeInvalid(current SessionMode) error {
	return xerrors.New(CodeUploadModeInvalid, "upload mode is invalid", xerrors.Details{
		"currentMode": string(current),
	})
}

func errUploadHashRequired(mode SessionMode) error {
	return xerrors.New(CodeUploadHashRequired, "content hash is required for upload mode", xerrors.Details{
		"uploadMode": string(mode),
		"field":      "contentHash",
	})
}

func errUploadHashInvalid(message string) error {
	return xerrors.New(CodeUploadHashInvalid, message, xerrors.Details{
		"field": "contentHash",
	})
}

func errUploadHashUnsupported(algorithm string) error {
	return xerrors.New(CodeUploadHashUnsupported, "content hash algorithm is unsupported", xerrors.Details{
		"field":     "contentHash.algorithm",
		"algorithm": algorithm,
	})
}

func errUploadCompletionOwnershipInvalid(message string) error {
	return xerrors.New(CodeUploadCompletionOwnershipInvalid, message, xerrors.Details{
		"field": "completion",
	})
}

func errUploadSessionStateConflict(current SessionStatus, allowed ...SessionStatus) error {
	values := make([]string, 0, len(allowed))
	for _, status := range allowed {
		values = append(values, string(status))
	}

	return xerrors.New(CodeUploadSessionStateConflict, "upload session state conflict", xerrors.Details{
		"currentStatus": string(current),
		"allowedStatus": values,
	})
}
