package tx

import "context"

type Manager interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}
