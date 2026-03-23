package queries

import "time"

type StoragePresignMetrics interface {
	RecordStoragePresignDuration(time.Duration)
}

type noopStoragePresignMetrics struct{}

func (noopStoragePresignMetrics) RecordStoragePresignDuration(time.Duration) {}
