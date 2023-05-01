package circuitbreaker

import (
	"errors"
	"sync"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrThresholdError    = errors.New("threshold must be >= 1")
	ErrResetTimeoutError = errors.New("reset timeout must be >= 1")
	ErrCircuitIsOpen     = errors.New("circuit is open")
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
)

var cbCache = newCache()

func New(serviceID string) Policy {
	return Policy{
		ServiceID:       serviceID,
		ThresholdErrors: 1,
		ResetTimeout:    time.Second * 1,
	}
}

func (p Policy) Run(cmd core.Command) (*Metric, error) {
	if err := p.validate(); err != nil {
		return nil, err
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
		m.FinishedAt = time.Now()
		return &m, ErrCircuitIsOpen
	}

	err := cmd()
	m.Error = err

	setPostState(p, cb, err)
	m.State = cb.State
	m.ErrorCount = cb.ErrorCount

	if p.AfterCircuitBreaker != nil {
		p.AfterCircuitBreaker(p, cb, err)
	}
	m.FinishedAt = time.Now()

	return &m, nil
}

func (p Policy) handledError(err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func (p Policy) validate() error {
	const minResetTimeout = time.Millisecond * 5
	switch {
	case p.ThresholdErrors < 1:
		return ErrThresholdError
	case p.ResetTimeout < minResetTimeout:
		return ErrResetTimeoutError
	default:
		return nil
	}
}

func setInitialState(p Policy, cb *CircuitBreaker) {
	circuitIsOpen := cb.State == OpenState
	shouldChangeToHalfOpen := time.Since(cb.TimeErrorOcurred) >= p.ResetTimeout

	if circuitIsOpen && shouldChangeToHalfOpen {
		halfOpenCircuit(p, cb)
	}
}

func setPostState(p Policy, cb *CircuitBreaker, err error) {
	if err != nil && !p.handledError(err) {
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

func newCache() *circuitBreakerCache {
	return &circuitBreakerCache{cache: make(map[string]*CircuitBreaker)}
}

func (m *Metric) ServiceID() string {
	return m.ID
}

func (m *Metric) PolicyDuration() time.Duration {
	return m.FinishedAt.Sub(m.StartedAt)
}

func (m *Metric) Success() bool {
	return m.Status == 0
}
