package refresh

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Refresher represents an entity in charge of maintaining an expiring value "fresh".
type Refresher[T any] interface {
	// WaitForInitialValue will return as soon as an initial value is loaded onto
	// the Refresher, or a timeout of the specified duration, whichever happens first.
	WaitForInitialValue(timeout time.Duration) error

	// GetCurrent returns the current value as a Refreshable.
	GetCurrent() *Refreshable[T]

	// GetNextRefreshTime returns the time at which the value will be refreshed next.
	GetNextRefreshTime() time.Time

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

// WithStorage is the refresher Option to set a mechanism for persisting a value
// in storage such that fresh values can be used across restarts of the application.
func WithStorage[T any](storage Storage[T]) Option[T] {
	return func(r *refresher[T]) { r.storage = storage }
}

// WithOnRefreshSuccess is the refresher Option to set a callback function to be fired
// after a succesful refreshing of the Refreshable.
func WithOnRefreshSuccess[T any](onRefreshSuccess func(*Refreshable[T])) Option[T] {
	return func(r *refresher[T]) { r.onRefreshSuccess = onRefreshSuccess }
}

// WithOnStorageReadSuccess is the refresher Option to set a callback function to be fired
// after a succesful reading of the Refreshable from storage.
func WithOnStorageReadSuccess[T any](onStorageReadSuccess func(*Refreshable[T])) Option[T] {
	return func(r *refresher[T]) { r.onStorageReadSuccess = onStorageReadSuccess }
}

// WithOnStorageWriteSuccess is the refresher Option to set a callback function to be fired
// after a succesful writing of the Refreshable to storage.
func WithOnStorageWriteSuccess[T any](onStorageWriteSuccess func(*Refreshable[T])) Option[T] {
	return func(r *refresher[T]) { r.onStorageWriteSuccess = onStorageWriteSuccess }
}

// WithOnRefreshFailure is the refresher Option to set a callback function to be fired
// after a failed refreshing of the Refreshable.
func WithOnRefreshFailure[T any](onRefreshFailure func(error)) Option[T] {
	return func(r *refresher[T]) { r.onRefreshFailure = onRefreshFailure }
}

// WithOnStorageReadFailure is the refresher Option to set a callback function to be fired
// after a failed reading from storage of the Refreshable.
func WithOnStorageReadFailure[T any](onStorageReadFailure func(error)) Option[T] {
	return func(r *refresher[T]) { r.onStorageReadFailure = onStorageReadFailure }
}

// WithOnStorageWriteFailure is the refresher Option to set a callback function to be fired
// after a failed writing to storage of the Refreshable.
func WithOnStorageWriteFailure[T any](onStorageWriteFailure func(error)) Option[T] {
	return func(r *refresher[T]) { r.onStorageWriteFailure = onStorageWriteFailure }
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

	storage Storage[T]

	// event handlers
	onRefreshSuccess      func(*Refreshable[T])
	onStorageReadSuccess  func(*Refreshable[T])
	onStorageWriteSuccess func(*Refreshable[T])
	onRefreshFailure      func(error)
	onStorageReadFailure  func(error)
	onStorageWriteFailure func(error)
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

		// event handlers
		onRefreshSuccess:      func(r *Refreshable[T]) { /* NOOP */ },
		onStorageReadSuccess:  func(r *Refreshable[T]) { /* NOOP */ },
		onStorageWriteSuccess: func(r *Refreshable[T]) { /* NOOP */ },
		onRefreshFailure:      func(err error) { /* NOOP */ },
		onStorageReadFailure:  func(err error) { /* NOOP */ },
		onStorageWriteFailure: func(err error) { /* NOOP */ },
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

	select {
	case <-time.After(timeout):
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
	r.RLock()
	defer r.RUnlock()
	return r.current
}

// Stop stops the refresher's go-routines and cleans up associated resources.
func (r *refresher[T]) Stop() {
	r.refreshCtxCancel()
}

// GetNextRefreshTime returns the time at which the value will be refreshed next.
func (r *refresher[T]) GetNextRefreshTime() time.Time {
	r.RLock()
	defer r.RUnlock()
	return r.refreshAt
}

// updateValue sets the current value of the Refreshable along with the refreshAt time.
func (r *refresher[T]) updateValue(newValue *Refreshable[T], refreshAt time.Time) {
	r.Lock()
	defer r.Unlock()
	r.current = newValue
	r.refreshAt = refreshAt
}

// refresh invokes the refresher's refreshFunc and updates its internal values.
func (r *refresher[T]) refresh(ctx context.Context) error {
	newValue, err := r.refreshFunc(ctx)
	if err != nil {
		return err
	}
	r.updateValue(newValue, r.refreshStrategy.GetRefreshAt(newValue))
	return nil
}

// store attempts to store the current value in Storage.
func (r *refresher[T]) store(ctx context.Context, refreshable *Refreshable[T]) {
	if r.storage == nil {
		return
	}
	if err := r.storage.Put(ctx, refreshable); err != nil {
		go r.onStorageWriteFailure(err)
		return
	}
}

// start is a long-lived routine which takes care of periodically
// invoking the refresher's refresh() method and handling its results.
//
// It also signals the initializationResult channel as soon as
// an initial value is retrieved and available.
func (r *refresher[T]) start(ctx context.Context) {

	// try retrieve from storage first
	if r.storage != nil {
		valueFromStorage, err := r.storage.Get(ctx)
		if err != nil {
			go r.onStorageReadFailure(err)
		} else {
			refreshAt := r.refreshStrategy.GetRefreshAt(valueFromStorage)

			// if the value is still fresh, we use it
			if time.Now().Before(refreshAt) {
				r.updateValue(valueFromStorage, refreshAt)
				r.initializationResult <- nil
			}
		}
	}

	// if the refresher has no value at this point, we need a fresh one.
	if r.GetCurrent() == nil {
		r.initializationResult <- r.refresh(ctx)
	}

	close(r.initializationResult) // channel is useless after the first write

	refreshTimer := time.NewTimer(time.Until(r.GetNextRefreshTime()))
	defer refreshTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return // stop
		case <-refreshTimer.C:
			if err := r.refresh(ctx); err != nil {
				refreshTimer.Reset(r.retryDelay)
				go r.onRefreshFailure(err)
				continue
			}
			nextRefreshIn := time.Until(r.GetNextRefreshTime())
			refreshTimer.Reset(nextRefreshIn)
			newValue := r.GetCurrent()
			go r.onRefreshSuccess(newValue)
			go r.store(ctx, newValue)
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
