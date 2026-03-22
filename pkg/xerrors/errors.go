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
	CodeInvalidArgument:    400,
	CodePayloadTooLarge:    413,
	CodeUnauthorized:       401,
	CodeForbidden:          403,
	CodeNotFound:           404,
	CodeConflict:           409,
	CodeRateLimited:        429,
	CodeInternalError:      500,
	CodeServiceUnavailable: 503,
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
