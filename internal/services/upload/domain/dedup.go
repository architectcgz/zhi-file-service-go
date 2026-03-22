package domain

type DedupDecision struct {
	Hit            bool
	BlobID         string
	FileID         string
	CanonicalETag  string
	CanonicalBytes int64
}

func (d DedupDecision) Validate() error {
	if !d.Hit {
		return nil
	}
	if d.BlobID == "" || d.FileID == "" {
		return errUploadHashInvalid("dedup hit must carry blob and file identifiers")
	}
	return nil
}

type CompletionResult struct {
	FileID   string
	BlobID   string
	DedupHit bool
}
