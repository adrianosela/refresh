package strategies

import (
	"time"

	"github.com/adrianosela/refresh"
)

type strategyStaticLifetimeSpent[T any] struct {
	lifetimeSpent time.Duration
}

// NewStaticLifetimeSpent returns a refresh.RefreshStrategy which will return a refresh time
// representing a static duration after the refresher's issuance.
func NewStaticLifetimeSpent[T any](lifetimeSpent time.Duration) refresh.RefreshStrategy[T] {
	return &strategyStaticLifetimeSpent[T]{lifetimeSpent: lifetimeSpent}
}

// GetRefreshAt returns the next refresh time for the Refreshable.
func (s *strategyStaticLifetimeSpent[T]) GetRefreshAt(refreshable *refresh.Refreshable[T]) time.Time {
	now := time.Now()

	refreshIfAfterTime := refreshable.IssuedAt.Add(s.lifetimeSpent)

	if now.Before(refreshIfAfterTime) {
		return refreshIfAfterTime
	}
	return now
}
