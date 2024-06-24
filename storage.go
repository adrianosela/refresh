package refresh

import (
	"context"
)

// Storage represents a mechanism for persisting values
// across restarts of an application using a Refresher.
type Storage[T any] interface {
	// Get retrieves a Refreshable.
	Get(context.Context) (*Refreshable[T], error)

	// Put stores a Refreshable.
	Put(context.Context, *Refreshable[T]) error
}

// storage is a Storage which runs inner
// functions to store and retrieve a Refreshable.
type storage[T any] struct {
	getFunc func(context.Context) (*Refreshable[T], error)
	putFunc func(context.Context, *Refreshable[T]) error
}

// Get retrieves a Refreshable by running the storage's inner getFunc.
func (s *storage[T]) Get(ctx context.Context) (*Refreshable[T], error) { return s.getFunc(ctx) }

// Put stores a Refreshable by running the storage's inner putFunc.
func (s *storage[T]) Put(ctx context.Context, r *Refreshable[T]) error { return s.putFunc(ctx, r) }

// StorageFromFunctions builds a functional Storage implementation.
func StorageFromFunctions[T any](
	getFunc func(context.Context) (*Refreshable[T], error),
	putFunc func(context.Context, *Refreshable[T]) error,
) Storage[T] {
	return &storage[T]{getFunc: getFunc, putFunc: putFunc}
}
