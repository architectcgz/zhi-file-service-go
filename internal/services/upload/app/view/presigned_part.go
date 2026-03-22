package view

import "time"

type PresignedPart struct {
	PartNumber int
	URL        string
	Headers    map[string]string
	ExpiresAt  time.Time
}

func CloneHeaders(headers map[string]string) map[string]string {
	return cloneHeaders(headers)
}
