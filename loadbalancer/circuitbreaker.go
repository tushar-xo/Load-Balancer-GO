package loadbalancer

import (
	"errors"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern for fault tolerance
// Prevents cascading failures by stopping calls to unhealthy services
type CircuitBreaker struct {
	name           string
	maxRequests    uint32
 interval        time.Duration
	timeout        time.Duration
	readyToTrip    func(counts Counts) bool
	onStateChange  func(name string, from CircuitBreakerState, to CircuitBreakerState)
	mutex          sync.RWMutex
	state          CircuitBreakerState
	generation     uint64
	counts         Counts
	expiry         time.Time
}

// Counts holds the statistics of the circuit breaker
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// CircuitBreakerOption configures CircuitBreaker
type CircuitBreakerOption func(*CircuitBreaker)

// WithMaxRequests sets the maximum number of requests allowed when the circuit breaker is half-open
func WithMaxRequests(maxRequests uint32) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		cb.maxRequests = maxRequests
	}
}

// WithInterval sets the cyclic period of the closed state for the circuit breaker to clear statistics
func WithInterval(interval time.Duration) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		cb.interval = interval
	}
}

// WithTimeout sets the timeout of the open state for the circuit breaker
func WithTimeout(timeout time.Duration) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		cb.timeout = timeout
	}
}

// WithReadyToTrip sets the criteria for tripping the circuit breaker
func WithReadyToTrip(readyToTrip func(counts Counts) bool) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		cb.readyToTrip = readyToTrip
	}
}

// WithOnStateChange sets the callback function to be called when the circuit breaker state changes
func WithOnStateChange(onStateChange func(name string, from CircuitBreakerState, to CircuitBreakerState)) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		cb.onStateChange = onStateChange
	}
}

// NewCircuitBreaker creates a new CircuitBreaker with the given name and options
func NewCircuitBreaker(name string, opts ...CircuitBreakerOption) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:        name,
		maxRequests: 1,
		interval:    60 * time.Second,
		timeout:     60 * time.Second,
		readyToTrip: defaultReadyToTrip,
		state:       StateClosed,
	}

	for _, opt := range opts {
		opt(cb)
	}

	return cb
}

// defaultReadyToTrip uses the default criteria for tripping the circuit breaker
func defaultReadyToTrip(counts Counts) bool {
	return counts.ConsecutiveFailures > 5
}

// Execute runs the given function if the circuit breaker is available
// It returns an error if the circuit breaker is open or the function fails
func (cb *CircuitBreaker) Execute(req func() (any, error)) (any, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result, err := req()
	cb.afterRequest(generation, err == nil)
	return result, err
}

// beforeRequest determines whether the circuit breaker allows the request
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()

	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	switch state {
	case StateOpen:
		return generation, ErrTooManyRequests
	case StateHalfOpen:
		if cb.counts.Requests >= cb.maxRequests {
			return generation, ErrTooManyRequests
		}
	}

	cb.counts.Requests++
	return generation, nil
}

// afterRequest processes the result of a request
func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

// onSuccess processes a successful request
func (cb *CircuitBreaker) onSuccess(state CircuitBreakerState, now time.Time) {
	cb.counts.TotalSuccesses++
	cb.counts.ConsecutiveSuccesses++
	cb.counts.ConsecutiveFailures = 0

	switch state {
	case StateClosed:
		// Do nothing, stay in closed state
	case StateHalfOpen:
		if cb.counts.Successes() >= cb.maxRequests {
			cb.setState(StateClosed, now)
		}
	}
}

// onFailure processes a failed request
func (cb *CircuitBreaker) onFailure(state CircuitBreakerState, now time.Time) {
	cb.counts.TotalFailures++
	cb.counts.ConsecutiveFailures++
	cb.counts.ConsecutiveSuccesses = 0

	switch state {
	case StateClosed:
		if cb.readyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}
	case StateHalfOpen:
		cb.setState(StateOpen, now)
	}
}

// currentState returns the current state of the circuit breaker
func (cb *CircuitBreaker) currentState(now time.Time) (CircuitBreakerState, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

// setState sets the state of the circuit breaker and calls the onStateChange callback
func (cb *CircuitBreaker) setState(state CircuitBreakerState, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prev, state)
	}
}

// toNewGeneration resets the counts and expiry of the circuit breaker
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts = Counts{}

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	default:
		cb.expiry = zero
	}
}

// Successes returns the number of successful requests in the current generation
func (c Counts) Successes() uint32 {
	return c.Requests - c.ConsecutiveFailures
}

// Failures returns the number of failed requests in the current generation
func (c Counts) Failures() uint32 {
	return c.ConsecutiveFailures
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	state, _ := cb.currentState(time.Now())
	return state
}

// Counts returns the internal counts of the circuit breaker
func (cb *CircuitBreaker) Counts() Counts {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return cb.counts
}

// Errors returned by the circuit breaker
var (
	ErrTooManyRequests = errors.New("circuit breaker is open")
	ErrServiceUnavailable = errors.New("service is unavailable")
)
