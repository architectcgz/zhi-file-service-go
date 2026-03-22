package xerrors

import (
	"errors"
	"testing"
)

func TestNew_ErrorFieldsAndStatus(t *testing.T) {
	t.Parallel()

	details := map[string]any{"field": "name"}
	err := New(CodeInvalidArgument, "invalid payload", details)

	if err.Code != CodeInvalidArgument {
		t.Fatalf("Code = %q, want %q", err.Code, CodeInvalidArgument)
	}
	if err.Message != "invalid payload" {
		t.Fatalf("Message = %q, want %q", err.Message, "invalid payload")
	}
	if got := err.Details["field"]; got != "name" {
		t.Fatalf("Details[field] = %v, want %q", got, "name")
	}
	if got := err.HTTPStatus(); got != 400 {
		t.Fatalf("HTTPStatus() = %d, want %d", got, 400)
	}
}

func TestWrap_UnwrapAndCode(t *testing.T) {
	t.Parallel()

	cause := errors.New("db unavailable")
	err := Wrap(CodeServiceUnavailable, "storage dependency unavailable", cause, map[string]any{"resourceType": "storage"})

	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(err, cause) = false, want true")
	}
	if got := CodeOf(err); got != CodeServiceUnavailable {
		t.Fatalf("CodeOf() = %q, want %q", got, CodeServiceUnavailable)
	}
	if got := StatusOf(err); got != 503 {
		t.Fatalf("StatusOf() = %d, want %d", got, 503)
	}
}

func TestStatusFromCode_DefaultsToInternal(t *testing.T) {
	t.Parallel()

	if got := StatusFromCode(Code("UNKNOWN_CODE")); got != 500 {
		t.Fatalf("StatusFromCode() = %d, want %d", got, 500)
	}
	if got := StatusOf(errors.New("plain error")); got != 500 {
		t.Fatalf("StatusOf() plain error = %d, want %d", got, 500)
	}
	if got := StatusOf(nil); got != 200 {
		t.Fatalf("StatusOf(nil) = %d, want %d", got, 200)
	}
}

func TestNew_ClonesDetails(t *testing.T) {
	t.Parallel()

	src := map[string]any{"a": "1"}
	err := New(CodeConflict, "conflict", src)
	src["a"] = "2"

	if got := err.Details["a"]; got != "1" {
		t.Fatalf("Details clone failed, got %v, want %q", got, "1")
	}
}
