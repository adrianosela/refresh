package refresh

import "time"

// RefreshStrategy represents a strategy to determine when a value should be refreshed.
type RefreshStrategy[T any] interface {
	// GetRefreshAt returns the time at which a Refreshable should be refreshed.
	// Any errors must be handled internally such that a valid time is returned.
	GetRefreshAt(refreshable *Refreshable[T]) time.Time
}

// RefreshAtFunc returns the time at which a Refreshable should be refreshed.
// Any errors must be handled internally such that a valid time is returned.
type RefreshAtFunc[T any] func(refreshable *Refreshable[T]) time.Time

// refreshStrategy is a RefreshStrategy which runs an inner
// function to determine the refresh time for a refreshable value.
type refreshStrategy[T any] struct {
	refreshAtFunc RefreshAtFunc[T]
}

// GetRefreshAt returns the time at which a Refreshable should be refreshed.
// Any errors must be handled internally such that a valid time is returned.
func (rs *refreshStrategy[T]) GetRefreshAt(refreshable *Refreshable[T]) time.Time {
	return rs.refreshAtFunc(refreshable)
}

// RefreshStrategyFromFunction builds a RefreshStrategy from a RefreshAtFunc.
func RefreshStrategyFromFunction[T any](refreshAtFunc RefreshAtFunc[T]) RefreshStrategy[T] {
	return &refreshStrategy[T]{refreshAtFunc: refreshAtFunc}
}
