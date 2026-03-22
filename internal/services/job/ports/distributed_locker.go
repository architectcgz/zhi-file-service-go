package ports

import (
	"context"
	"time"
)

type DistributedLocker interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (LockHandle, bool, error)
}

type LockHandle interface {
	Key() string
	Owner() string
	Renew(ctx context.Context, ttl time.Duration) error
	Release(ctx context.Context) error
}
