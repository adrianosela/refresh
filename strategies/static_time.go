package strategies

import (
	"time"

	"github.com/adrianosela/refresh"
)

type strategyStaticTime[T any] struct {
	time time.Time
}

// NewStaticTime returns a refresh.RefreshStrategy which will
// return a refresh time representing a static time (timestamp).
func NewStaticTime[T any](time time.Time) refresh.RefreshStrategy[T] {
	return &strategyStaticTime[T]{time: time}
}

// GetRefreshAt returns the next refresh time for the Refreshable.
func (s *strategyStaticTime[T]) GetRefreshAt(refreshable *refresh.Refreshable[T]) time.Time {
	return s.time
}
