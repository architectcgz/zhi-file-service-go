package xerrors

import (
	"errors"
	"testing"
)

func TestNewCreatesIndependentDetailsMap(t *testing.T) {
	original := Details{"field": "tenantId"}
	err := New("INVALID_ARGUMENT", "invalid tenant", original)
	original["field"] = "mutated"

	if err.Code != "INVALID_ARGUMENT" {
		t.Fatalf("expected code INVALID_ARGUMENT, got %s", err.Code)
	}

	if got := err.Details["field"]; got != "tenantId" {
		t.Fatalf("expected cloned details, got %v", got)
	}
}

func TestWrapPreservesCauseAndCode(t *testing.T) {
	cause := errors.New("db timeout")
	err := Wrap("SERVICE_UNAVAILABLE", "database unavailable", cause, Details{"retryable": true})

	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped error to match original cause")
	}

	if got := CodeOf(err); got != "SERVICE_UNAVAILABLE" {
		t.Fatalf("expected code SERVICE_UNAVAILABLE, got %s", got)
	}
}

func TestCodeOfReturnsEmptyWhenCanonicalErrorMissing(t *testing.T) {
	if got := CodeOf(errors.New("plain error")); got != "" {
		t.Fatalf("expected empty code, got %s", got)
	}
}
