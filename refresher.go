package refresh

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Refresher represents an entity in charge of maintaing an expiring value "fresh".
type Refresher[T any] interface {
	// WaitForInitialValue will return as soon as an initial value is loaded onto
	// the Refresher, or a timeout of the specified duration, whichever happens first.
	WaitForInitialValue(timeout time.Duration) error

	// GetCurrent returns the current value and whether it is fresh i.e. not expired.
	GetCurrent() *Refreshable[T]

	// Stop stops the Refresher's go-routines and cleans up associated resources.
	Stop()
}

// Refreshable represents a refreshable value.
type Refreshable[T any] struct {
	Value     T
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// RefreshFunc returns a new value as well as when it expires. If a non-nil error is returned,
// both the value and the time will be ignored and their current value will be maintained.
type RefreshFunc[T any] func(context.Context) (*Refreshable[T], error)

// RefreshAtFunc returns the time at which a Refreshable should be refreshed.
// Any errors must be handled internally such that a valid time is returned.
type RefreshAtFunc[T any] func(refreshable *Refreshable[T]) time.Time

// Option represents a refresher configuration option.
type Option[T any] func(*refresher[T])

// WithRetryDelay is the refresher Option to override the default refresh-failure retry delay.
func WithRetryDelay[T any](retryDelay time.Duration) Option[T] {
	return func(r *refresher[T]) { r.retryDelay = retryDelay }
}

// WithRefreshStrategy is the refresher Option to provide a non-default RefreshStrategy
// used to calculate when a recently acquired value should be refreshed next.
func WithRefreshStrategy[T any](refreshStrategy RefreshStrategy[T]) Option[T] {
	return func(r *refresher[T]) { r.refreshStrategy = refreshStrategy }
}

// refresher is the private, default implementation of the Refresher interface.
type refresher[T any] struct {
	sync.RWMutex

	// managed with private getters wrapping the mutex
	current   *Refreshable[T]
	refreshAt time.Time

	// managed by Stop()
	refreshCtxCancel context.CancelFunc

	// managed by start()
	initializationResult chan error

	refreshFunc     RefreshFunc[T]
	refreshStrategy RefreshStrategy[T]
	retryDelay      time.Duration
}

// NewRefresher returns a Refresher initialized with the given RefreshFunc and Option(s).
// The recommended usage is to call WaitForInitialValue(<timeout>) immediately afterwards.
func NewRefresher[T any](refreshFunc RefreshFunc[T], opts ...Option[T]) Refresher[T] {
	ref := &refresher[T]{
		refreshFunc:          refreshFunc,
		current:              nil,
		refreshAt:            time.Now(),
		initializationResult: make(chan error),

		// default option values
		retryDelay:      time.Minute * 15,
		refreshStrategy: RefreshStrategyFromFunction(defaultRefreshStrategyFunc[T]),
	}
	for _, opt := range opts {
		opt(ref)
	}

	refreshCtx, refreshCtxCancel := context.WithCancel(context.Background())
	ref.refreshCtxCancel = refreshCtxCancel

	go ref.start(refreshCtx)

	return ref
}

// WaitForInitialValue will return as soon as an initial value is loaded onto
// the refresher, or a timeout of the specified duration, whichever happens first.
func (r *refresher[T]) WaitForInitialValue(timeout time.Duration) error {
	if r.GetCurrent() != nil {
		return nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		return fmt.Errorf("timed out after %s waiting for initial value", timeout)
	case err := <-r.initializationResult:
		if err != nil {
			return fmt.Errorf("failed to acquire initial value: %v", err)
		}
		return nil
	}
}

// GetCurrent returns the current value.
func (r *refresher[T]) GetCurrent() *Refreshable[T] {
	return r.getCurrent()
}

// Stop stops the refresher's go-routines and cleans up associated resources.
func (r *refresher[T]) Stop() {
	r.refreshCtxCancel()
}

// getCurrent is the private getter for the current value.
func (r *refresher[T]) getCurrent() *Refreshable[T] {
	r.RLock()
	defer r.RUnlock()
	return r.current
}

// getNextRefreshTime is the private getter for the refreshAt time.
func (r *refresher[T]) getNextRefreshTime() time.Time {
	r.RLock()
	defer r.RUnlock()
	return r.refreshAt
}

// refresh invokes the refresher's refreshFunc and updates its internal values.
func (r *refresher[T]) refresh(ctx context.Context) error {
	newValue, err := r.refreshFunc(ctx)
	if err != nil {
		return err
	}
	refreshAt := r.refreshStrategy.GetRefreshAt(newValue)
	r.Lock()
	defer r.Unlock()
	r.current = newValue
	r.refreshAt = refreshAt
	return nil
}

// start is a long-lived routine which takes care of periodically
// invoking the refresher's refresh() method and handling its results.
//
// It also signals the initializationResult channel as soon as
// an initial value is retrieved and available.
func (r *refresher[T]) start(ctx context.Context) {
	r.initializationResult <- r.refresh(ctx)
	close(r.initializationResult) // channel is useless after the first write

	refreshTimer := time.NewTimer(time.Until(r.getNextRefreshTime()))
	defer refreshTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return // stop
		case <-refreshTimer.C:
			if err := r.refresh(ctx); err != nil {
				refreshTimer.Reset(r.retryDelay)
				continue
			}
			nextRefreshIn := time.Until(r.getNextRefreshTime())
			refreshTimer.Reset(nextRefreshIn)
		}
	}
}

func defaultRefreshStrategyFunc[T any](refreshable *Refreshable[T]) time.Time {
	// if value is already expired, refresh now
	if time.Now().After(refreshable.ExpiresAt) {
		return time.Now()
	}

	lifetimeSoFarSeconds := time.Since(refreshable.IssuedAt).Seconds()
	lifetimeTotalSeconds := refreshable.ExpiresAt.Sub(refreshable.IssuedAt).Seconds()
	twoThirdsOfTotalLifetimeSeconds := lifetimeTotalSeconds * 2 / 3

	// already exceeded 66% of lifetime, refresh now
	if lifetimeSoFarSeconds > twoThirdsOfTotalLifetimeSeconds {
		return time.Now()
	}

	// otherwise refresh at 66% of its lifetime
	return refreshable.IssuedAt.Add(time.Duration(twoThirdsOfTotalLifetimeSeconds) * time.Second)
}
