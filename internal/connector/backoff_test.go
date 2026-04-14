package connector

import (
	"testing"
	"time"
)

func TestBackoffNextCapsAndReset(t *testing.T) {
	backoff := NewBackoff(500*time.Millisecond, 2*time.Second)

	got := []time.Duration{
		backoff.Next(),
		backoff.Next(),
		backoff.Next(),
		backoff.Next(),
	}
	want := []time.Duration{
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		2 * time.Second,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step %d: expected %s, got %s", i, want[i], got[i])
		}
	}

	backoff.Reset()
	if next := backoff.Next(); next != 500*time.Millisecond {
		t.Fatalf("expected reset to return base delay, got %s", next)
	}
}
