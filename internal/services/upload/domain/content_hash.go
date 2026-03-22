package domain

import (
	"regexp"
	"strings"
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type ContentHash struct {
	Algorithm string
	Value     string
}

func (h ContentHash) Normalize() ContentHash {
	h.Algorithm = strings.ToUpper(strings.TrimSpace(h.Algorithm))
	h.Value = strings.ToLower(strings.TrimSpace(h.Value))
	return h
}

func (h ContentHash) Validate() error {
	normalized := h.Normalize()
	if normalized.Algorithm == "" || normalized.Value == "" {
		return errUploadHashInvalid("content hash algorithm and value are required")
	}
	if normalized.Algorithm != "SHA256" {
		return errUploadHashUnsupported(normalized.Algorithm)
	}
	if !sha256Pattern.MatchString(normalized.Value) {
		return errUploadHashInvalid("content hash value must be lowercase hex sha256")
	}
	return nil
}
