package commands

type CompleteUploadMetrics interface {
	RecordDedupHit()
	RecordDedupMiss()
}

type noopCompleteUploadMetrics struct{}

func (noopCompleteUploadMetrics) RecordDedupHit()  {}
func (noopCompleteUploadMetrics) RecordDedupMiss() {}
