package ports

import "context"

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}
