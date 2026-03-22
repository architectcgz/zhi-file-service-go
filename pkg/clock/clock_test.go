package clock

import (
	"testing"
	"time"
)

func TestFixedClockNowReturnsConfiguredTime(t *testing.T) {
	expected := time.Date(2026, 3, 22, 13, 0, 0, 0, time.FixedZone("CST", 8*3600))
	c := NewFixed(expected)

	if got := c.Now(); !got.Equal(expected.UTC()) {
		t.Fatalf("expected %s, got %s", expected.UTC(), got)
	}
}

func TestSystemClockNowUsesUTC(t *testing.T) {
	got := SystemClock{}.Now()

	if got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %s", got.Location())
	}
}
