package observability

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func NewLogger(serviceName string, level string) (*slog.Logger, error) {
	parsedLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parsedLevel,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "ts"
			case slog.MessageKey:
				attr.Key = "message"
			}
			return attr
		},
	})
	return slog.New(handler).With("service", serviceName), nil
}

func NewBootstrapLogger(serviceName string) *slog.Logger {
	logger, err := NewLogger(serviceName, "error")
	if err == nil {
		return logger
	}

	fallback := slog.NewJSONHandler(os.Stdout, nil)
	return slog.New(fallback).With("service", serviceName)
}

func parseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level: %q", raw)
	}
}
