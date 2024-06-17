package refresh

import (
	"context"
)

// Storage represents a mechanism for persisting values
// across restarts of an application using a Refresher.
type Storage[T any] interface {
	Get(context.Context) (*Refreshable[T], error)
	Put(context.Context, *Refreshable[T]) error
}
