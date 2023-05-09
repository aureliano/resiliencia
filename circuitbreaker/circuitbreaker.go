package circuitbreaker

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	// Policy threshold is less than minimum required.
	ErrThresholdValidation = fmt.Errorf("threshold must be >= %d", MinThresholdErrors)

	// Policy reset timeout is less than minimum required.
	ErrResetTimeoutValidation = fmt.Errorf("reset timeout must be >= %dms", MinResetTimeout.Milliseconds())

	// No command nor wrapped policy is set.
	ErrCommandRequired = errors.New("command nor wrapped policy provided")

	// Circuit breaker is open and accesses to service are not allowed.
	ErrCircuitIsOpen = errors.New("circuit is open")

	// No circuit breaker found to the given service.
	ErrCircuitBreakerNotFound = errors.New("no circuit breaker found")
)

// Policy defines the circuit breaker algorithm execution policy.
type Policy struct {
	// The registered service id.
	ServiceID string

	// Number of errors until the circuit breaker is open.
	ThresholdErrors int

	// How long to wait to change the circuit state to HalfOpen.
	ResetTimeout time.Duration

	// Expected erros (not expected errors will open the circuit breaker immediately).
	Errors []error

	// Function called before execution.
	BeforeCircuitBreaker func(p Policy, status *CircuitBreaker)

	// Function called after execution.
	AfterCircuitBreaker func(p Policy, status *CircuitBreaker, err error)

	// Function called when circuit breaker is just open.
	OnOpenCircuit func(p Policy, status *CircuitBreaker, err error)

	// Function called when circuit is just half open.
	OnHalfOpenCircuit func(p Policy, status *CircuitBreaker)

	// Function called when circuit is just closed.
	OnClosedCircuit func(p Policy, status *CircuitBreaker)

	// The command supplier.
	Command core.Command

	// Any policy that will be wrapped by this one.
	Policy core.PolicySupplier
}

// Metric keeps the running state of the circuit breaker.
type Metric struct {
	// The registered service id.
	ID string

	// The execution status (success is non zero).
	Status int

	// When execution started.
	StartedAt time.Time

	// When execution finished.
	FinishedAt time.Time

	// The error (if execution wasn't succeeded)
	Error error

	// Circuit breaker state (closed, open or half open).
	State CircuitState

	// How many errors occurred.
	ErrorCount int
}

// CircuitState is the circuit breaker state.
type CircuitState int

// CircuitBreaker is the abstraction of a circuit breaker policy.
type CircuitBreaker struct {
	// Circuit breaker state (closed, open or half open).
	State CircuitState

	// When error occurred.
	TimeErrorOcurred time.Time

	// How many errors occurred.
	ErrorCount int
}

type circuitBreakerCache struct {
	mu    sync.Mutex
	cache map[string]*CircuitBreaker
}

const (
	// Indicates that circuit breaker is healthy and the requests will be fulfilled.
	ClosedState = CircuitState(0)

	// Indicates that circuit breaker is unhealthy and no requests will be serviced.
	OpenState = CircuitState(1)

	// Indicates that circuit breaker is in a state between healthy and unhealthy.
	HalfOpenState = CircuitState(2)

	// Minimum expected to be set on ResetTimeout field of a circuit breaker policy.
	MinResetTimeout = time.Millisecond * 5

	// Minimum expected to be set on ThresholdErrors field of a circuit breaker policy.
	MinThresholdErrors = 1
)

var cbCache = newCache()

// State queries for a circuit breaker state by service id.
//
// Returns the state of a circuit breaker or an error if no circuit breaker is found.
//
// Possible error(s): ErrCircuitBreakerNotFound
func State(p Policy) (CircuitState, error) {
	cbCache.mu.Lock()
	defer cbCache.mu.Unlock()

	cb := cbCache.cache[p.ServiceID]
	if cb == nil {
		return -1, ErrCircuitBreakerNotFound
	}
	setInitialState(p, cb)

	return cb.State, nil
}

// New creates a circuit breaker policy with default values set.
func New(serviceID string) Policy {
	return Policy{
		ServiceID:       serviceID,
		ThresholdErrors: MinThresholdErrors,
		ResetTimeout:    time.Second * 1,
	}
}

// Run executes a command supplier or a wrapped policy in a circuit breaker.
//
// Possible error(s): ErrThresholdValidation, ErrResetTimeoutValidation, ErrCommandRequiredErrCircuitIsOpen.
func (p Policy) Run(metric core.Metric) error {
	if err := validate(p); err != nil {
		return err
	}

	cbCache.mu.Lock()
	cb := cbCache.cache[p.ServiceID]
	if cb == nil {
		cb = new(CircuitBreaker)
		cbCache.cache[p.ServiceID] = cb
	}
	cbCache.mu.Unlock()

	m := Metric{ID: p.ServiceID, StartedAt: time.Now()}
	if p.BeforeCircuitBreaker != nil {
		p.BeforeCircuitBreaker(p, cb)
	}

	setInitialState(p, cb)
	m.State = cb.State
	m.ErrorCount = cb.ErrorCount

	if cb.State == OpenState {
		m.Status = 1
		m.Error = ErrCircuitIsOpen
		m.FinishedAt = time.Now()
		metric[reflect.TypeOf(m).String()] = m

		return ErrCircuitIsOpen
	}

	err := execute(p, metric)

	if err != nil {
		m.Error = err
		m.Status = 1
	}

	setPostState(p, cb, err)
	m.State = cb.State
	m.ErrorCount = cb.ErrorCount

	if p.AfterCircuitBreaker != nil {
		p.AfterCircuitBreaker(p, cb, err)
	}
	m.FinishedAt = time.Now()
	metric[reflect.TypeOf(m).String()] = m

	return nil
}

// WithCommand encapsulates this policy in a new policy with given command supplier.
func (p Policy) WithCommand(command core.Command) core.PolicySupplier {
	p.Command = command
	return p
}

// WithPolicy encapsulates this policy in a new policy with wrapped policy.
func (p Policy) WithPolicy(policy core.PolicySupplier) core.PolicySupplier {
	p.Policy = policy
	return p
}

func execute(p Policy, metric core.Metric) error {
	if p.Command != nil && p.Policy == nil {
		return p.Command()
	}

	return p.Policy.Run(metric)
}

func setInitialState(p Policy, cb *CircuitBreaker) {
	circuitIsOpen := cb.State == OpenState
	shouldChangeToHalfOpen := time.Since(cb.TimeErrorOcurred) >= p.ResetTimeout

	if circuitIsOpen && shouldChangeToHalfOpen {
		halfOpenCircuit(p, cb)
	}
}

func setPostState(p Policy, cb *CircuitBreaker, err error) {
	if err != nil {
		expectedError := handledError(p, err)
		cb.ErrorCount++

		if cb.ErrorCount >= p.ThresholdErrors || !expectedError {
			openCircuit(p, cb, err)
		}
	} else if cb.State == HalfOpenState {
		closeCircuit(p, cb)
	}
}

func openCircuit(p Policy, cb *CircuitBreaker, err error) {
	cb.State = OpenState
	cb.TimeErrorOcurred = time.Now()

	if p.OnOpenCircuit != nil {
		p.OnOpenCircuit(p, cb, err)
	}
}

func closeCircuit(p Policy, cb *CircuitBreaker) {
	cb.State = ClosedState
	cb.ErrorCount = 0

	if p.OnClosedCircuit != nil {
		p.OnClosedCircuit(p, cb)
	}
}

func halfOpenCircuit(p Policy, cb *CircuitBreaker) {
	cb.State = HalfOpenState

	if p.OnHalfOpenCircuit != nil {
		p.OnHalfOpenCircuit(p, cb)
	}
}

func handledError(p Policy, err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func validate(p Policy) error {
	switch {
	case p.ThresholdErrors < MinThresholdErrors:
		return ErrThresholdValidation
	case p.ResetTimeout < MinResetTimeout:
		return ErrResetTimeoutValidation
	case p.Command == nil && p.Policy == nil:
		return ErrCommandRequired
	default:
		return nil
	}
}

func newCache() *circuitBreakerCache {
	return &circuitBreakerCache{cache: make(map[string]*CircuitBreaker)}
}

// ServiceID returns the service id registered to the policy binded to this metric.
func (m Metric) ServiceID() string {
	return m.ID
}

// PolicyDuration returns the policy execution duration.
// In short, finished at less (-) started at.
func (m Metric) PolicyDuration() time.Duration {
	return m.FinishedAt.Sub(m.StartedAt)
}

// Success returns whether the policy execution succeeded or not.
// In short, status is zero and error is nil.
func (m Metric) Success() bool {
	return (m.Status == 0) && (m.Error == nil)
}
