package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDGeneratesAndInjectsWhenMissing(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(FromContext(r.Context())))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	headerID := resp.Header.Get(headerRequestID)
	if headerID == "" {
		t.Fatalf("expected generated request id")
	}
	if string(body) != headerID {
		t.Fatalf("request id in context and header mismatch: %s vs %s", string(body), headerID)
	}
}

func TestRequestIDPreservesIncomingHeader(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(FromContext(r.Context())))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(headerRequestID, "fixed-id")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Result().Header.Get(headerRequestID); got != "fixed-id" {
		t.Fatalf("unexpected header request id: %s", got)
	}
}
