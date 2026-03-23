package xerrors

import "fmt"

// Code is the machine-readable error code shared across services.
type Code string

const (
	CodeInvalidArgument    Code = "INVALID_ARGUMENT"
	CodePayloadTooLarge    Code = "PAYLOAD_TOO_LARGE"
	CodeUnauthorized       Code = "UNAUTHORIZED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeNotFound           Code = "NOT_FOUND"
	CodeConflict           Code = "CONFLICT"
	CodeRateLimited        Code = "RATE_LIMITED"
	CodeInternalError      Code = "INTERNAL_ERROR"
	CodeServiceUnavailable Code = "SERVICE_UNAVAILABLE"
)

var httpStatusByCode = map[Code]int{
	CodeInvalidArgument:                         400,
	CodePayloadTooLarge:                         413,
	CodeUnauthorized:                            401,
	CodeForbidden:                               403,
	CodeNotFound:                                404,
	CodeConflict:                                409,
	CodeRateLimited:                             429,
	CodeInternalError:                           500,
	CodeServiceUnavailable:                      503,
	Code("FILE_NOT_FOUND"):                      404,
	Code("FILE_ACCESS_DENIED"):                  403,
	Code("ACCESS_TICKET_INVALID"):               404,
	Code("ACCESS_TICKET_EXPIRED"):               404,
	Code("DOWNLOAD_NOT_ALLOWED"):                403,
	Code("TENANT_SCOPE_DENIED"):                 403,
	Code("ADMIN_PERMISSION_DENIED"):             403,
	Code("TENANT_NOT_FOUND"):                    404,
	Code("TENANT_STATUS_INVALID"):               409,
	Code("TENANT_POLICY_INVALID"):               400,
	Code("AUDIT_QUERY_INVALID"):                 400,
	Code("UPLOAD_SESSION_NOT_FOUND"):            404,
	Code("UPLOAD_SESSION_STATE_CONFLICT"):       409,
	Code("UPLOAD_COMPLETE_IN_PROGRESS"):         409,
	Code("UPLOAD_MODE_INVALID"):                 400,
	Code("UPLOAD_HASH_REQUIRED"):                400,
	Code("UPLOAD_HASH_INVALID"):                 400,
	Code("UPLOAD_HASH_UNSUPPORTED"):             400,
	Code("UPLOAD_HASH_MISMATCH"):                409,
	Code("UPLOAD_PARTS_MISSING"):                409,
	Code("UPLOAD_MULTIPART_NOT_FOUND"):          409,
	Code("UPLOAD_MULTIPART_CONFLICT"):           409,
	Code("TENANT_QUOTA_EXCEEDED"):               409,
	Code("MIME_TYPE_NOT_ALLOWED"):               400,
	Code("UPLOAD_COMPLETION_OWNERSHIP_INVALID"): 400,
}

// Details carries structured error metadata.
type Details map[string]any

// Error is the canonical application error model.
type Error struct {
	Code    Code
	Message string
	Details Details
	Err     error
}

// New creates a canonical error without a wrapped cause.
func New(code Code, message string, details Details) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: cloneDetails(details),
	}
}

// Wrap creates a canonical error around an underlying cause.
func Wrap(code Code, message string, err error, details Details) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: cloneDetails(details),
		Err:     err,
	}
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Err == nil {
		return string(e.Code) + ": " + e.Message
	}

	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

// HTTPStatus resolves the canonical HTTP status for this error.
func (e *Error) HTTPStatus() int {
	if e == nil {
		return StatusFromCode(CodeInternalError)
	}

	return StatusFromCode(e.Code)
}

// Unwrap exposes the underlying cause to errors.Is/errors.As.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// CodeOf returns the canonical error code if present.
func CodeOf(err error) Code {
	for err != nil {
		if coded, ok := err.(*Error); ok {
			return coded.Code
		}

		type unwrapper interface {
			Unwrap() error
		}

		unwrapped, ok := err.(unwrapper)
		if !ok {
			return ""
		}

		err = unwrapped.Unwrap()
	}

	return ""
}

// StatusFromCode returns the canonical HTTP status for the given error code.
func StatusFromCode(code Code) int {
	if status, ok := httpStatusByCode[code]; ok {
		return status
	}

	return httpStatusByCode[CodeInternalError]
}

// StatusOf returns the canonical HTTP status for the given error value.
func StatusOf(err error) int {
	if err == nil {
		return 200
	}

	code := CodeOf(err)
	if code == "" {
		return StatusFromCode(CodeInternalError)
	}

	return StatusFromCode(code)
}

func cloneDetails(details Details) Details {
	if len(details) == 0 {
		return nil
	}

	cloned := make(Details, len(details))
	for key, value := range details {
		cloned[key] = value
	}

	return cloned
}
