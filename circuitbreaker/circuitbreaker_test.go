package circuitbreaker_test

import (
	"errors"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/circuitbreaker"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	p := circuitbreaker.New("backend-service-name")
	assert.Equal(t, "backend-service-name", p.ServiceID)
	assert.Equal(t, 1, p.ThresholdErrors)
	assert.Equal(t, time.Second*1, p.ResetTimeout)
}

func TestRunValidatePolicyThresholdErrors(t *testing.T) {
	p := circuitbreaker.Policy{ThresholdErrors: 0, ResetTimeout: time.Second * 1}
	_, err := p.Run(func() error { return nil })

	assert.ErrorIs(t, err, circuitbreaker.ErrThresholdError)
}

func TestRunValidatePolicyResetTimeout(t *testing.T) {
	p := circuitbreaker.Policy{ThresholdErrors: 1, ResetTimeout: time.Millisecond * 1}
	_, err := p.Run(func() error { return nil })

	assert.ErrorIs(t, err, circuitbreaker.ErrResetTimeoutError)
}

func TestRunCircuitIsOpen(t *testing.T) {
	circuitbreaker.Reset()
	errTest := errors.New("err test")

	p := circuitbreaker.Policy{
		ServiceID:            "backend-service-name",
		ThresholdErrors:      1,
		ResetTimeout:         time.Second * 1,
		BeforeCircuitBreaker: func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker) {},
		OnOpenCircuit:        func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {},
		OnHalfOpenCircuit:    func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker) {},
		OnClosedCircuit:      func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker) {},
		AfterCircuitBreaker:  func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {},
	}

	m, err := p.Run(func() error { return errTest })
	assert.Nil(t, err)

	assert.Equal(t, "backend-service-name", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest)
	assert.EqualValues(t, circuitbreaker.OpenState, m.State)
	assert.Equal(t, 1, m.ErrorCount)
	assert.Equal(t, "backend-service-name", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())

	m, err = p.Run(func() error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitIsOpen)

	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.EqualValues(t, circuitbreaker.OpenState, m.State)
	assert.Equal(t, 1, m.ErrorCount)
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}

func TestRunCircuitHalfOpenSetToClosed(t *testing.T) {
	circuitbreaker.Reset()
	errTest := errors.New("err test")

	var state circuitbreaker.CircuitState
	p := circuitbreaker.Policy{
		ServiceID:            "backend-service-name",
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

	m, err := p.Run(func() error { return errTest })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	assert.Equal(t, "backend-service-name", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest)
	assert.Equal(t, "backend-service-name", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())

	m, err = p.Run(func() error { return nil })
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitIsOpen)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	assert.Equal(t, "backend-service-name", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "backend-service-name", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())

	time.Sleep(time.Millisecond * 300)
	m, err = p.Run(func() error { return nil })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "backend-service-name", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunHandledErrors(t *testing.T) {
	circuitbreaker.Reset()
	var state circuitbreaker.CircuitState

	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := circuitbreaker.Policy{
		ServiceID:       "backend-service-name",
		ThresholdErrors: 1,
		ResetTimeout:    time.Millisecond * 300,
		Errors:          []error{errTest1, errTest2},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	m, err := p.Run(func() error { return errTest1 })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest1)
	assert.Equal(t, "backend-service-name", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())

	m, err = p.Run(func() error { return errTest2 })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest2)
	assert.Equal(t, "backend-service-name", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunUnhandledError(t *testing.T) {
	circuitbreaker.Reset()
	var state circuitbreaker.CircuitState

	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := circuitbreaker.Policy{
		ServiceID:       "backend-service-name",
		ThresholdErrors: 1,
		ResetTimeout:    time.Millisecond * 300,
		Errors:          []error{errTest1},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	m, err := p.Run(func() error { return errTest1 })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest1)
	assert.Equal(t, "backend-service-name", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())

	m, err = p.Run(func() error { return errTest2 })
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	assert.Equal(t, "backend-service-name", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest2)
	assert.Equal(t, "backend-service-name", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}
