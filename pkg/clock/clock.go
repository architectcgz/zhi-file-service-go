package clock

import "time"

// Clock provides the current time for services that need deterministic tests.
type Clock interface {
	Now() time.Time
}

// SystemClock uses the local process clock.
type SystemClock struct{}

// Now returns the current UTC time.
func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

// FixedClock always returns the same time.
type FixedClock struct {
	current time.Time
}

// NewFixed creates a fixed clock normalized to UTC.
func NewFixed(current time.Time) FixedClock {
	return FixedClock{current: current.UTC()}
}

// Now returns the configured fixed time.
func (c FixedClock) Now() time.Time {
	return c.current
}
