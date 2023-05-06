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
	ErrThresholdError         = fmt.Errorf("threshold must be >= %d", MinThresholdErrors)
	ErrResetTimeoutError      = fmt.Errorf("reset timeout must be >= %dms", MinResetTimeout.Milliseconds())
	ErrCommandRequiredError   = errors.New("command is required")
	ErrCircuitIsOpen          = errors.New("circuit is open")
	ErrCircuitBreakerNotFound = errors.New("no circuit breaker found")
)

type Policy struct {
	ServiceID            string
	ThresholdErrors      int
	ResetTimeout         time.Duration
	Errors               []error
	BeforeCircuitBreaker func(p Policy, status *CircuitBreaker)
	AfterCircuitBreaker  func(p Policy, status *CircuitBreaker, err error)
	OnOpenCircuit        func(p Policy, status *CircuitBreaker, err error)
	OnHalfOpenCircuit    func(p Policy, status *CircuitBreaker)
	OnClosedCircuit      func(p Policy, status *CircuitBreaker)
	Command              core.Command
}

type Metric struct {
	ID         string
	Status     int
	StartedAt  time.Time
	FinishedAt time.Time
	Error      error
	State      CircuitState
	ErrorCount int
}

type CircuitState int

type CircuitBreaker struct {
	State            CircuitState
	TimeErrorOcurred time.Time
	ErrorCount       int
}

type circuitBreakerCache struct {
	mu    sync.Mutex
	cache map[string]*CircuitBreaker
}

const (
	ClosedState   = 0
	OpenState     = 1
	HalfOpenState = 2

	MinResetTimeout    = time.Millisecond * 5
	MinThresholdErrors = 1
)

var cbCache = newCache()

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

func New(serviceID string) Policy {
	return Policy{
		ServiceID:       serviceID,
		ThresholdErrors: MinThresholdErrors,
		ResetTimeout:    time.Second * 1,
	}
}

func (p Policy) Run() (core.MetricRecorder, error) {
	if p.Command == nil {
		return nil, ErrCommandRequiredError
	}

	metric := core.NewMetric()
	err := runPolicy(metric, p, func() (core.MetricRecorder, error) { return nil, p.Command() })
	m := metric[reflect.TypeOf(Metric{}).String()]

	return m, err
}

func (p Policy) RunPolicy(metric core.Metric, supplier core.PolicySupplier) error {
	return runPolicy(metric, p, supplier.Run)
}

func runPolicy(metric core.Metric, parent Policy, yield func() (core.MetricRecorder, error)) error {
	if err := validate(parent); err != nil {
		return err
	}

	cbCache.mu.Lock()
	cb := cbCache.cache[parent.ServiceID]
	if cb == nil {
		cb = new(CircuitBreaker)
		cbCache.cache[parent.ServiceID] = cb
	}
	cbCache.mu.Unlock()

	m := Metric{ID: parent.ServiceID, StartedAt: time.Now()}
	if parent.BeforeCircuitBreaker != nil {
		parent.BeforeCircuitBreaker(parent, cb)
	}

	setInitialState(parent, cb)
	m.State = cb.State
	m.ErrorCount = cb.ErrorCount

	if cb.State == OpenState {
		m.Status = 1
		m.Error = ErrCircuitIsOpen
		m.FinishedAt = time.Now()
		metric[reflect.TypeOf(m).String()] = m

		return ErrCircuitIsOpen
	}

	mr, err := yield()
	if mr != nil {
		metric[reflect.TypeOf(mr).String()] = mr
	}

	if err != nil {
		m.Error = err
		m.Status = 1
	}

	setPostState(parent, cb, err)
	m.State = cb.State
	m.ErrorCount = cb.ErrorCount

	if parent.AfterCircuitBreaker != nil {
		parent.AfterCircuitBreaker(parent, cb, err)
	}
	m.FinishedAt = time.Now()
	metric[reflect.TypeOf(m).String()] = m

	return nil
}

func setInitialState(p Policy, cb *CircuitBreaker) {
	circuitIsOpen := cb.State == OpenState
	shouldChangeToHalfOpen := time.Since(cb.TimeErrorOcurred) >= p.ResetTimeout

	if circuitIsOpen && shouldChangeToHalfOpen {
		halfOpenCircuit(p, cb)
	}
}

func setPostState(p Policy, cb *CircuitBreaker, err error) {
	if err != nil && !handledError(p, err) {
		openCircuit(p, cb, err)
	} else if cb.State == HalfOpenState {
		closeCircuit(p, cb)
	}
}

func openCircuit(p Policy, cb *CircuitBreaker, err error) {
	cb.State = OpenState
	cb.TimeErrorOcurred = time.Now()
	cb.ErrorCount++

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
		return ErrThresholdError
	case p.ResetTimeout < MinResetTimeout:
		return ErrResetTimeoutError
	default:
		return nil
	}
}

func newCache() *circuitBreakerCache {
	return &circuitBreakerCache{cache: make(map[string]*CircuitBreaker)}
}

func (m Metric) ServiceID() string {
	return m.ID
}

func (m Metric) PolicyDuration() time.Duration {
	return m.FinishedAt.Sub(m.StartedAt)
}

func (m Metric) Success() bool {
	return (m.Status == 0) && (m.Error == nil)
}
