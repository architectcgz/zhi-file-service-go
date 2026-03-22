package runner

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	platformredis "github.com/architectcgz/zhi-file-service-go/internal/platform/redis"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisLockerAcquireRenewRelease(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := platformredis.NewClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}))
	t.Cleanup(func() {
		_ = client.Close()
	})

	locker := NewRedisLocker(client)
	handle, acquired, err := locker.Acquire(context.Background(), "job:test", 5*time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if !acquired || handle == nil {
		t.Fatalf("Acquire() = %#v, %v, want acquired handle", handle, acquired)
	}

	if err := handle.Renew(context.Background(), 10*time.Second); err != nil {
		t.Fatalf("Renew() error = %v", err)
	}
	ttl := mr.TTL("job:test")
	if ttl < 9*time.Second {
		t.Fatalf("TTL = %s, want >= 9s", ttl)
	}

	if err := handle.Release(context.Background()); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if mr.Exists("job:test") {
		t.Fatal("expected lock key to be deleted")
	}
}

func TestRedisLockerSkipsWhenAlreadyLocked(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := platformredis.NewClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}))
	t.Cleanup(func() {
		_ = client.Close()
	})

	locker := NewRedisLocker(client)
	firstHandle, acquired, err := locker.Acquire(context.Background(), "job:test", 5*time.Second)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	if !acquired || firstHandle == nil {
		t.Fatalf("first Acquire() = %#v, %v, want acquired handle", firstHandle, acquired)
	}

	secondHandle, acquired, err := locker.Acquire(context.Background(), "job:test", 5*time.Second)
	if err != nil {
		t.Fatalf("second Acquire() error = %v", err)
	}
	if acquired {
		t.Fatal("expected second acquire to miss")
	}
	if secondHandle != nil {
		t.Fatalf("second handle = %#v, want nil", secondHandle)
	}
}
