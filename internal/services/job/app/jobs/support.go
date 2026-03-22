package jobs

const defaultBatchSize = 100

func normalizeBatchSize(value int) int {
	if value <= 0 {
		return defaultBatchSize
	}

	return value
}
