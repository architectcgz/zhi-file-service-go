package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const headerRequestID = "X-Request-Id"

type requestIDContextKey struct{}

var fallbackCounter atomic.Uint64

func RequestID(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(headerRequestID))
		if requestID == "" {
			requestID = generateRequestID()
		}

		w.Header().Set(headerRequestID, requestID)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), requestID)))
	})
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func FromContext(ctx context.Context) string {
	value, ok := ctx.Value(requestIDContextKey{}).(string)
	if !ok {
		return ""
	}
	return value
}

func generateRequestID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	count := fallbackCounter.Add(1)
	return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano))) + "-" + strconv.FormatUint(count, 10)
}
