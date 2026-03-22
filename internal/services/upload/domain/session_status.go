package domain

type SessionStatus string

const (
	SessionStatusInitiated  SessionStatus = "INITIATED"
	SessionStatusUploading  SessionStatus = "UPLOADING"
	SessionStatusCompleting SessionStatus = "COMPLETING"
	SessionStatusCompleted  SessionStatus = "COMPLETED"
	SessionStatusAborted    SessionStatus = "ABORTED"
	SessionStatusExpired    SessionStatus = "EXPIRED"
	SessionStatusFailed     SessionStatus = "FAILED"
)

func (s SessionStatus) Validate() error {
	switch s {
	case SessionStatusInitiated,
		SessionStatusUploading,
		SessionStatusCompleting,
		SessionStatusCompleted,
		SessionStatusAborted,
		SessionStatusExpired,
		SessionStatusFailed:
		return nil
	default:
		return errUploadSessionStateConflict(s)
	}
}

func (s SessionStatus) IsTerminal() bool {
	switch s {
	case SessionStatusCompleted, SessionStatusAborted, SessionStatusExpired, SessionStatusFailed:
		return true
	default:
		return false
	}
}
