package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	platformredis "github.com/architectcgz/zhi-file-service-go/internal/platform/redis"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	goredis "github.com/redis/go-redis/v9"
)

var (
	ErrRedisRequired     = errors.New("redis client is required")
	ErrLockTTLRequired   = errors.New("lock ttl must be > 0")
	errLockOwnershipLost = errors.New("distributed lock ownership lost")
)

var renewScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) ~= ARGV[1] then
  return 0
end
return redis.call("pexpire", KEYS[1], ARGV[2])
`)

var releaseScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) ~= ARGV[1] then
  return 0
end
return redis.call("del", KEYS[1])
`)

type RedisLocker struct {
	client *goredis.Client
	owner  string
}

type redisLockHandle struct {
	client *goredis.Client
	key    string
	owner  string
}

func NewRedisLocker(client *platformredis.Client) *RedisLocker {
	if client == nil || client.Raw() == nil {
		return &RedisLocker{}
	}

	host, _ := os.Hostname()
	host = strings.TrimSpace(host)
	if host == "" {
		host = "unknown"
	}

	return &RedisLocker{
		client: client.Raw(),
		owner:  fmt.Sprintf("%s:%d", host, os.Getpid()),
	}
}

func (l *RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration) (ports.LockHandle, bool, error) {
	if l == nil || l.client == nil {
		return nil, false, ErrRedisRequired
	}
	if ttl <= 0 {
		return nil, false, ErrLockTTLRequired
	}

	key = strings.TrimSpace(key)
	acquired, err := l.client.SetNX(ctx, key, l.owner, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, nil
	}

	return &redisLockHandle{
		client: l.client,
		key:    key,
		owner:  l.owner,
	}, true, nil
}

func (h *redisLockHandle) Key() string {
	if h == nil {
		return ""
	}
	return h.key
}

func (h *redisLockHandle) Owner() string {
	if h == nil {
		return ""
	}
	return h.owner
}

func (h *redisLockHandle) Renew(ctx context.Context, ttl time.Duration) error {
	if h == nil || h.client == nil {
		return ErrRedisRequired
	}
	if ttl <= 0 {
		return ErrLockTTLRequired
	}

	result, err := renewScript.Run(ctx, h.client, []string{h.key}, h.owner, ttl.Milliseconds()).Int64()
	if err != nil {
		return err
	}
	if result == 0 {
		return errLockOwnershipLost
	}
	return nil
}

func (h *redisLockHandle) Release(ctx context.Context) error {
	if h == nil || h.client == nil {
		return ErrRedisRequired
	}

	result, err := releaseScript.Run(ctx, h.client, []string{h.key}, h.owner).Int64()
	if err != nil {
		return err
	}
	if result == 0 {
		return errLockOwnershipLost
	}
	return nil
}
