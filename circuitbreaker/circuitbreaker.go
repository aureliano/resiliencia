package circuitbreaker

import (
	"context"
	"errors"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrThresholdError    = errors.New("threshold must be >= 1")
	ErrResetTimeoutError = errors.New("reset timeout must be >= 1")
	ErrCircuitIsOpen     = errors.New("circuit is open")
)

type Policy struct {
	ThresholdErrors      int
	ResetTimeout         time.Duration
	Errors               []error
	BeforeCircuitBreaker func(p Policy, status CircuitBreaker)
	AfterCircuitBreaker  func(p Policy, status CircuitBreaker, err error)
	OnOpenCircuit        func(p Policy, status CircuitBreaker, err error)
	OnHalfOpenCircuit    func(p Policy, status CircuitBreaker)
	OnClosedCircuit      func(p Policy, status CircuitBreaker)
}

type CircuitState int

type CircuitBreaker struct {
	State            CircuitState
	TimeErrorOcurred time.Time
	ErrorCount       int
}

const (
	ClosedState   = 0
	OpenState     = 1
	HalfOpenState = 2
)

var cbState = CircuitBreaker{}

func Reset() {
	cbState = CircuitBreaker{}
}

func New() Policy {
	return Policy{
		ThresholdErrors: 1,
		ResetTimeout:    time.Second * 1,
	}
}

func (p Policy) Run(ctx context.Context, cmd core.Command) error {
	if err := p.validate(); err != nil {
		return err
	}

	if p.BeforeCircuitBreaker != nil {
		p.BeforeCircuitBreaker(p, cbState)
	}

	setInitialState(p)

	if cbState.State == OpenState {
		return ErrCircuitIsOpen
	}

	err := cmd(ctx)

	setPostState(p, err)

	if p.AfterCircuitBreaker != nil {
		p.AfterCircuitBreaker(p, cbState, err)
	}

	return nil
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

func setInitialState(p Policy) {
	circuitIsOpen := cbState.State == OpenState
	shouldChangeToHalfOpen := time.Since(cbState.TimeErrorOcurred) >= p.ResetTimeout

	if circuitIsOpen && shouldChangeToHalfOpen {
		halfOpenCircuit(p)
	}
}

func setPostState(p Policy, err error) {
	if err != nil && !p.handledError(err) {
		openCircuit(p, err)
	} else if cbState.State == HalfOpenState {
		closeCircuit(p)
	}
}

func openCircuit(p Policy, err error) {
	cbState.State = OpenState
	cbState.TimeErrorOcurred = time.Now()
	cbState.ErrorCount++

	if p.OnOpenCircuit != nil {
		p.OnOpenCircuit(p, cbState, err)
	}
}

func closeCircuit(p Policy) {
	cbState.State = ClosedState
	cbState.ErrorCount = 0

	if p.OnClosedCircuit != nil {
		p.OnClosedCircuit(p, cbState)
	}
}

func halfOpenCircuit(p Policy) {
	cbState.State = HalfOpenState

	if p.OnHalfOpenCircuit != nil {
		p.OnHalfOpenCircuit(p, cbState)
	}
}
