package strategies

import (
	"math/rand"
	"time"

	"github.com/adrianosela/refresh"
)

type strategyRandomWithinLifetimeWindow[T any] struct {
	min float64
	max float64
}

// NewRandomWithinLifetimeWindow returns a refresh.RefreshStrategy which will return a refresh time
// somewhere within a window of the Refreshable's lifetime that lies within a minimum and maximum
// % of the lifetime.
//
// For example, with min = 0.50 and max = 0.75, the GetRefreshAt method will return a refresh time
// somewhere where the refresher's lifespan is 50%-75% elapsed.
//
// The randomness is useful as it serves as jitter to prevent en-masse Refreshable refreshes.
//
// It is required that both min and max are within [0.01, 0.99], and that min <= max.
// - If min or max are < 0.01, they will be overridden to 0.01
// - If min or max are > 0.99, they will be overridden to 0.99
// - If min > max, it will be overridden to max
func NewRandomWithinLifetimeWindow[T any](min, max float64) refresh.RefreshStrategy[T] {
	min = clamp(min, 0.01, 0.99)
	max = clamp(max, 0.01, 0.99)
	if min > max {
		min = max
	}
	return &strategyRandomWithinLifetimeWindow[T]{min: min, max: max}
}

func clamp(value, lowerBound, upperBound float64) float64 {
	if value < lowerBound {
		return lowerBound
	}
	if value > upperBound {
		return upperBound
	}
	return value
}

// GetRefreshAt returns the next refresh time for the Refreshable.
func (s *strategyRandomWithinLifetimeWindow[T]) GetRefreshAt(refreshable *refresh.Refreshable[T]) time.Time {

	now := time.Now()

	// if value is already expired, refresh now
	if now.After(refreshable.ExpiresAt) {
		return now
	}

	lifetimeSoFarSeconds := now.Sub(refreshable.IssuedAt).Seconds()
	lifetimeTotalSeconds := refreshable.ExpiresAt.Sub(refreshable.IssuedAt).Seconds()
	randomFactorInWindow := s.min + rand.Float64()*(s.max-s.min)
	desiredElapsedLifetimeSeconds := lifetimeTotalSeconds * randomFactorInWindow

	// already exceeded desired elapsed lifetime, refresh now
	if lifetimeSoFarSeconds > desiredElapsedLifetimeSeconds {
		return now
	}

	// otherwise refresh at the desired elapsed lifetime
	return refreshable.IssuedAt.Add(time.Duration(desiredElapsedLifetimeSeconds) * time.Second)
}
