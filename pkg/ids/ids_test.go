package ids

import (
	"math/rand"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
	"github.com/oklog/ulid/v2"
)

func TestGeneratorProducesValidULID(t *testing.T) {
	generator := NewGenerator(
		clock.NewFixed(time.Date(2026, 3, 22, 13, 30, 0, 0, time.UTC)),
		ulid.Monotonic(rand.New(rand.NewSource(1)), 0),
	)

	id, err := generator.New()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(id) != 26 {
		t.Fatalf("expected ULID length 26, got %d", len(id))
	}

	if _, err := ulid.Parse(id); err != nil {
		t.Fatalf("expected parsable ulid, got %v", err)
	}
}

func TestGeneratorProducesOrderedIDsAtSameTimestamp(t *testing.T) {
	generator := NewGenerator(
		clock.NewFixed(time.Date(2026, 3, 22, 13, 30, 0, 0, time.UTC)),
		ulid.Monotonic(rand.New(rand.NewSource(1)), 0),
	)

	first, err := generator.New()
	if err != nil {
		t.Fatalf("expected no error for first id, got %v", err)
	}

	second, err := generator.New()
	if err != nil {
		t.Fatalf("expected no error for second id, got %v", err)
	}

	if !(first < second) {
		t.Fatalf("expected lexicographic order, got first=%s second=%s", first, second)
	}
}
