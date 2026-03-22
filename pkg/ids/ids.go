package ids

import (
	"fmt"
	"io"
	"sync"

	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/oklog/ulid/v2"
)

// Generator defines the shared ID generation contract.
type Generator interface {
	New() (string, error)
}

// ULIDGenerator generates time-ordered ULIDs.
type ULIDGenerator struct {
	clock   clock.Clock
	entropy io.Reader
	mu      sync.Mutex
}

// NewGenerator creates a ULID generator with explicit dependencies.
func NewGenerator(c clock.Clock, entropy io.Reader) *ULIDGenerator {
	if c == nil {
		c = clock.SystemClock{}
	}
	if entropy == nil {
		entropy = ulid.DefaultEntropy()
	}

	return &ULIDGenerator{
		clock:   c,
		entropy: entropy,
	}
}

// New creates a new ULID string.
func (g *ULIDGenerator) New() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	id, err := ulid.New(ulid.Timestamp(g.clock.Now()), g.entropy)
	if err != nil {
		return "", fmt.Errorf("generate ulid: %w", err)
	}

	return id.String(), nil
}

// New returns a ULID string using the default process-wide generator.
func New() (string, error) {
	return defaultGenerator.New()
}

// MustNew returns a ULID string or panics if generation fails.
func MustNew() string {
	id, err := New()
	if err != nil {
		panic(err)
	}

	return id
}

var defaultGenerator = NewGenerator(clock.SystemClock{}, ulid.DefaultEntropy())
