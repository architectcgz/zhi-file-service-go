package storage

import "errors"

var (
	ErrObjectNotFound        = errors.New("storage: object not found")
	ErrMultipartNotFound     = errors.New("storage: multipart upload not found")
	ErrMultipartConflict     = errors.New("storage: multipart conflict")
	ErrPreconditionFailed    = errors.New("storage: precondition failed")
	ErrProviderUnavailable   = errors.New("storage: provider unavailable")
	ErrInvalidBucketConfig   = errors.New("storage: invalid bucket config")
	ErrInvalidPresignRequest = errors.New("storage: invalid presign request")
)
