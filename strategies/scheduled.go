package strategies

import (
	"time"

	"github.com/adrianosela/refresh"
)

var (
	maxTime = time.Unix(1<<63-1, 0)
)

type strategyScheduled[T any] struct {
	times []time.Time
}

// NewScheduled returns a refresh.RefreshStrategy which will
// return a refresh time representing the closest time in the
// future out of a given list of timestamps on which to refresh.
func NewScheduled[T any](times ...time.Time) refresh.RefreshStrategy[T] {
	return &strategyScheduled[T]{times: times}
}

// GetRefreshAt returns the next refresh time for the Refreshable.
func (s *strategyScheduled[T]) GetRefreshAt(refreshable *refresh.Refreshable[T]) time.Time {
	now := time.Now()
	for _, t := range s.times {
		if t.After(now) {
			return t
		}
	}
	// all given refresh times already occurred... never refresh again
	return maxTime
}
