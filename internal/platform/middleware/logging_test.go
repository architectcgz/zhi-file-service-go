package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"
)

func TestLoggingSuppressesSuccessfulRequestsAtInfoLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := Logging(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if buffer.Len() != 0 {
		t.Fatalf("expected no log output for success request at info level, got: %s", buffer.String())
	}
}

func TestLoggingEmitsServerErrorsAtInfoLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := Logging(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if !strings.Contains(buffer.String(), "\"status\":500") {
		t.Fatalf("expected status in log output, got: %s", buffer.String())
	}
	if !strings.Contains(buffer.String(), "\"msg\":\"http_request\"") {
		t.Fatalf("expected http_request log message, got: %s", buffer.String())
	}
}
