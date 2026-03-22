package observability

import (
	"sync"
	"time"
)

type JobHealth struct {
	JobName        string
	Status         Status
	LastStartedAt  time.Time
	LastFinishedAt time.Time
	LastDuration   time.Duration
	LastError      string
	ItemsProcessed int
	RetryCount     int
	LockAcquired   bool
}

type OutboxHealth struct {
	Status         Status
	LastStartedAt  time.Time
	LastFinishedAt time.Time
	LastDuration   time.Duration
	LastError      string
	Claimed        int
	Published      int
	Failed         int
	RetryCount     int
}

type Snapshot struct {
	Jobs   map[string]JobHealth
	Outbox OutboxHealth
}

type MemoryHealthStore struct {
	mu     sync.RWMutex
	jobs   map[string]JobHealth
	outbox OutboxHealth
}

func NewMemoryHealthStore() *MemoryHealthStore {
	return &MemoryHealthStore{
		jobs: make(map[string]JobHealth),
	}
}

func (s *MemoryHealthStore) RecordJob(health JobHealth) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.jobs == nil {
		s.jobs = make(map[string]JobHealth)
	}
	s.jobs[health.JobName] = health
}

func (s *MemoryHealthStore) RecordOutbox(health OutboxHealth) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.outbox = health
}

func (s *MemoryHealthStore) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make(map[string]JobHealth, len(s.jobs))
	for jobName, health := range s.jobs {
		jobs[jobName] = health
	}

	return Snapshot{
		Jobs:   jobs,
		Outbox: s.outbox,
	}
}
