package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	goredis "github.com/redis/go-redis/v9"
)

const redisKeyPrefix = "access:ticket:idempotency:"

type MemoryStore struct {
	clock clock.Clock

	mu      sync.Mutex
	records map[string]memoryRecord
}

type memoryRecord struct {
	record    ports.AccessTicketIssueRecord
	expiresAt time.Time
}

func NewMemoryStore(clk clock.Clock) *MemoryStore {
	if clk == nil {
		clk = clock.SystemClock{}
	}

	return &MemoryStore{
		clock:   clk,
		records: make(map[string]memoryRecord),
	}
}

func (s *MemoryStore) Get(_ context.Context, key string) (ports.AccessTicketIssueRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	record, ok := s.records[key]
	if !ok {
		return ports.AccessTicketIssueRecord{}, false, nil
	}

	return record.record, true, nil
}

func (s *MemoryStore) PutIfAbsent(_ context.Context, key string, record ports.AccessTicketIssueRecord, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()
	if _, exists := s.records[key]; exists {
		return false, nil
	}

	s.records[key] = memoryRecord{
		record:    record,
		expiresAt: s.clock.Now().Add(normalizeTTL(ttl)),
	}
	return true, nil
}

func (s *MemoryStore) cleanupExpiredLocked() {
	now := s.clock.Now()
	for key, record := range s.records {
		if !record.expiresAt.After(now) {
			delete(s.records, key)
		}
	}
}

type RedisStore struct {
	client *goredis.Client
}

func NewRedisStore(client *goredis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Get(ctx context.Context, key string) (ports.AccessTicketIssueRecord, bool, error) {
	value, err := s.client.Get(ctx, redisKey(key)).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return ports.AccessTicketIssueRecord{}, false, nil
		}
		return ports.AccessTicketIssueRecord{}, false, fmt.Errorf("get access ticket idempotency record: %w", err)
	}

	var record ports.AccessTicketIssueRecord
	if err := json.Unmarshal(value, &record); err != nil {
		return ports.AccessTicketIssueRecord{}, false, fmt.Errorf("unmarshal access ticket idempotency record: %w", err)
	}

	return record, true, nil
}

func (s *RedisStore) PutIfAbsent(ctx context.Context, key string, record ports.AccessTicketIssueRecord, ttl time.Duration) (bool, error) {
	payload, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("marshal access ticket idempotency record: %w", err)
	}

	stored, err := s.client.SetNX(ctx, redisKey(key), payload, normalizeTTL(ttl)).Result()
	if err != nil {
		return false, fmt.Errorf("put access ticket idempotency record: %w", err)
	}
	return stored, nil
}

func normalizeTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return time.Second
	}
	return ttl
}

func redisKey(key string) string {
	return redisKeyPrefix + key
}

var _ ports.AccessTicketIdempotencyStore = (*MemoryStore)(nil)
var _ ports.AccessTicketIdempotencyStore = (*RedisStore)(nil)
