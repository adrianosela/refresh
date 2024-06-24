package strategies

import (
	"time"

	"github.com/adrianosela/refresh"
)

type strategyStaticLifetimeLeft[T any] struct {
	lifetimeLeft time.Duration
}

// NewStaticLifetimeLeft returns a refresh.RefreshStrategy which will return a refresh time
// representing a static duration before the refresher's expiry.
func NewStaticLifetimeLeft[T any](lifetimeLeft time.Duration) refresh.RefreshStrategy[T] {
	return &strategyStaticLifetimeLeft[T]{lifetimeLeft: lifetimeLeft}
}

// GetRefreshAt returns the next refresh time for the Refreshable.
func (s *strategyStaticLifetimeLeft[T]) GetRefreshAt(refreshable *refresh.Refreshable[T]) time.Time {
	now := time.Now()

	refreshIfAfterTime := refreshable.ExpiresAt.Add(-s.lifetimeLeft)

	if now.Before(refreshIfAfterTime) {
		return refreshIfAfterTime
	}
	return now
}
