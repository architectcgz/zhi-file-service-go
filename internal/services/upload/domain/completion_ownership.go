package domain

type CompletionOwnership string

const (
	CompletionOwnershipAcquired      CompletionOwnership = "ACQUIRED"
	CompletionOwnershipHeldByCaller  CompletionOwnership = "HELD_BY_CALLER"
	CompletionOwnershipHeldByAnother CompletionOwnership = "HELD_BY_ANOTHER"
	CompletionOwnershipAlreadyDone   CompletionOwnership = "ALREADY_COMPLETED"
)

func (o CompletionOwnership) OwnsCompletion() bool {
	return o == CompletionOwnershipAcquired || o == CompletionOwnershipHeldByCaller
}
