package llm

import (
	"sync"
	"time"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation; requests pass through.
	CircuitOpen                         // Too many failures; requests are blocked.
	CircuitHalfOpen                     // One probe request allowed to test recovery.
)

// CircuitBreaker implements the circuit breaker pattern to protect against
// cascading failures when an LLM provider is down.
type CircuitBreaker struct {
	mu           sync.Mutex
	state        CircuitState
	failures     int
	threshold    int
	resetTimeout time.Duration
	lastFailure  time.Time
}

// NewCircuitBreaker creates a circuit breaker that opens after threshold
// consecutive failures and resets after resetTimeout.
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        CircuitClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// Allow returns true if a request should be permitted through the breaker.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) >= cb.resetTimeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		// Only one probe request at a time; block until it completes.
		return false
	default:
		return true
	}
}

// RecordSuccess resets the breaker to closed state.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
}

// RecordFailure increments the failure count and opens the circuit if threshold is reached.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.threshold {
		cb.state = CircuitOpen
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
