package circuitbreaker_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	circuitbreaker "github.com/aureliano/resiliencia/circuit_breaker"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	p := circuitbreaker.New()
	assert.Equal(t, 1, p.ThresholdErrors)
	assert.Equal(t, time.Second*1, p.ResetTimeout)
}

func TestRunValidatePolicyThresholdErrors(t *testing.T) {
	p := circuitbreaker.Policy{ThresholdErrors: 0, ResetTimeout: time.Second * 1}
	err := p.Run(context.TODO(), func(ctx context.Context) error { return nil })

	assert.ErrorIs(t, err, circuitbreaker.ErrThresholdError)
}

func TestRunValidatePolicyResetTimeout(t *testing.T) {
	p := circuitbreaker.Policy{ThresholdErrors: 1, ResetTimeout: time.Millisecond * 1}
	err := p.Run(context.TODO(), func(ctx context.Context) error { return nil })

	assert.ErrorIs(t, err, circuitbreaker.ErrResetTimeoutError)
}

func TestRunCircuitIsOpen(t *testing.T) {
	circuitbreaker.Reset()
	p := circuitbreaker.Policy{
		ThresholdErrors:      1,
		ResetTimeout:         time.Second * 1,
		BeforeCircuitBreaker: func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker) {},
		OnOpenCircuit:        func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {},
		OnHalfOpenCircuit:    func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker) {},
		OnClosedCircuit:      func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker) {},
		AfterCircuitBreaker:  func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {},
	}

	err := p.Run(context.TODO(), func(ctx context.Context) error { return fmt.Errorf("any error") })
	assert.Nil(t, err)

	err = p.Run(context.TODO(), func(ctx context.Context) error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitIsOpen)
}

func TestRunCircuitHalfOpenSetToClosed(t *testing.T) {
	circuitbreaker.Reset()
	var state circuitbreaker.CircuitState
	p := circuitbreaker.Policy{
		ThresholdErrors:      1,
		ResetTimeout:         time.Millisecond * 300,
		BeforeCircuitBreaker: func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker) {},
		OnOpenCircuit:        func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {},
		OnHalfOpenCircuit:    func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker) {},
		OnClosedCircuit:      func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker) {},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	err := p.Run(context.TODO(), func(ctx context.Context) error { return fmt.Errorf("any error") })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	err = p.Run(context.TODO(), func(ctx context.Context) error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitIsOpen)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	time.Sleep(time.Millisecond * 300)
	err = p.Run(context.TODO(), func(ctx context.Context) error { return nil })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)
}

func TestRunHandledErrors(t *testing.T) {
	circuitbreaker.Reset()
	var state circuitbreaker.CircuitState

	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := circuitbreaker.Policy{
		ThresholdErrors: 1,
		ResetTimeout:    time.Millisecond * 300,
		Errors:          []error{errTest1, errTest2},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	err := p.Run(context.TODO(), func(ctx context.Context) error { return errTest1 })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	err = p.Run(context.TODO(), func(ctx context.Context) error { return errTest2 })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)
}

func TestRunUnhandledError(t *testing.T) {
	circuitbreaker.Reset()
	var state circuitbreaker.CircuitState

	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := circuitbreaker.Policy{
		ThresholdErrors: 1,
		ResetTimeout:    time.Millisecond * 300,
		Errors:          []error{errTest1},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	err := p.Run(context.TODO(), func(ctx context.Context) error { return errTest1 })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	err = p.Run(context.TODO(), func(ctx context.Context) error { return errTest2 })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)
}
