package connector

import "time"

type Backoff struct {
	base    time.Duration
	max     time.Duration
	current time.Duration
}

func NewBackoff(base, max time.Duration) *Backoff {
	return &Backoff{base: base, max: max}
}

func (b *Backoff) Next() time.Duration {
	if b.current == 0 {
		b.current = b.base
		return b.current
	}
	b.current *= 2
	if b.current > b.max {
		b.current = b.max
	}
	return b.current
}

func (b *Backoff) Reset() {
	b.current = 0
}
