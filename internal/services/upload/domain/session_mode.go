package domain

type SessionMode string

const (
	SessionModeInline          SessionMode = "INLINE"
	SessionModePresignedSingle SessionMode = "PRESIGNED_SINGLE"
	SessionModeDirect          SessionMode = "DIRECT"
)

func (m SessionMode) Validate() error {
	switch m {
	case SessionModeInline, SessionModePresignedSingle, SessionModeDirect:
		return nil
	default:
		return errUploadModeInvalid(m)
	}
}

func (m SessionMode) RequiresContentHash() bool {
	return m == SessionModePresignedSingle || m == SessionModeDirect
}

func (m SessionMode) RequiresProviderUploadID() bool {
	return m == SessionModeDirect
}

func (m SessionMode) DefaultTotalParts() int {
	if m == SessionModeDirect {
		return 0
	}
	return 1
}
