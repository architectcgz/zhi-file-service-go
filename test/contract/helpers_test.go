package contract_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func newRequest(t *testing.T, method string, url string, body string) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	return req
}

func doRequest(t *testing.T, client *http.Client, req *http.Request) *http.Response {
	t.Helper()

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	return resp
}

func decodeJSONResponse(t *testing.T, body io.Reader) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	return payload
}

