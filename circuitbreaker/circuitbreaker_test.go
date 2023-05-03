package circuitbreaker_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/circuitbreaker"
	"github.com/aureliano/resiliencia/core"
	"github.com/stretchr/testify/assert"
)

func TestPolicyImplementsPolicySupplier(t *testing.T) {
	p := circuitbreaker.New("service-name")
	i := reflect.TypeOf((*core.PolicySupplier)(nil)).Elem()

	assert.True(t, reflect.TypeOf(p).Implements(i))
}

func TestMetricImplementsMetricRecorder(t *testing.T) {
	m := circuitbreaker.Metric{}
	i := reflect.TypeOf((*core.MetricRecorder)(nil)).Elem()

	assert.True(t, reflect.TypeOf(m).Implements(i))
}

func TestCircuitBreakerState(t *testing.T) {
	state, err := circuitbreaker.State(circuitbreaker.Policy{ServiceID: "unknown"})
	assert.EqualValues(t, -1, state)
	assert.Equal(t, err, circuitbreaker.ErrCircuitBreakerNotFound)

	p := circuitbreaker.Policy{
		ServiceID:       "service-name",
		ThresholdErrors: 1,
		ResetTimeout:    time.Millisecond * 50,
	}

	p.Command = func() error { return nil }
	_, err = p.Run()
	assert.Nil(t, err)
	state, err = circuitbreaker.State(p)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	p.Command = func() error { return fmt.Errorf("any") }
	_, err = p.Run()
	assert.Nil(t, err)
	state, err = circuitbreaker.State(p)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	time.Sleep(time.Millisecond * 50)
	state, err = circuitbreaker.State(p)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.HalfOpenState, state)

	p.Command = func() error { return nil }
	_, err = p.Run()
	assert.Nil(t, err)
	state, err = circuitbreaker.State(p)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)
}

func TestNew(t *testing.T) {
	p := circuitbreaker.New("backend-service-name")
	assert.Equal(t, "backend-service-name", p.ServiceID)
	assert.Equal(t, 1, p.ThresholdErrors)
	assert.Equal(t, time.Second*1, p.ResetTimeout)
}

func TestRunValidatePolicyThresholdErrors(t *testing.T) {
	p := circuitbreaker.Policy{ThresholdErrors: 0, ResetTimeout: time.Second * 1}
	p.Command = func() error { return nil }
	_, err := p.Run()

	assert.ErrorIs(t, err, circuitbreaker.ErrThresholdError)
}

func TestRunValidatePolicyResetTimeout(t *testing.T) {
	p := circuitbreaker.Policy{ThresholdErrors: 1, ResetTimeout: time.Millisecond * 1}
	p.Command = func() error { return nil }
	_, err := p.Run()

	assert.ErrorIs(t, err, circuitbreaker.ErrResetTimeoutError)
}

func TestRunCircuitIsOpen(t *testing.T) {
	errTest := errors.New("err test")

	p := circuitbreaker.Policy{
		ServiceID:            "backend-service-name-1",
		ThresholdErrors:      1,
		ResetTimeout:         time.Second * 1,
		BeforeCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnOpenCircuit:        func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
		OnHalfOpenCircuit:    func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnClosedCircuit:      func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		AfterCircuitBreaker:  func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
	}
	p.Command = func() error { return errTest }

	r, err := p.Run()
	m, _ := r.(*circuitbreaker.Metric)
	assert.Nil(t, err)

	assert.Equal(t, "backend-service-name-1", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest)
	assert.EqualValues(t, circuitbreaker.OpenState, m.State)
	assert.Equal(t, 1, m.ErrorCount)
	assert.Equal(t, "backend-service-name-1", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())

	p.Command = func() error { return nil }
	r, err = p.Run()
	m, _ = r.(*circuitbreaker.Metric)
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
	errTest := errors.New("err test")

	var state circuitbreaker.CircuitState
	p := circuitbreaker.Policy{
		ServiceID:            "backend-service-name-2",
		ThresholdErrors:      1,
		ResetTimeout:         time.Millisecond * 300,
		BeforeCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnOpenCircuit:        func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
		OnHalfOpenCircuit:    func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnClosedCircuit:      func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	p.Command = func() error { return errTest }
	r, err := p.Run()
	m, _ := r.(*circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	assert.Equal(t, "backend-service-name-2", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest)
	assert.Equal(t, "backend-service-name-2", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())

	p.Command = func() error { return nil }
	r, err = p.Run()
	m, _ = r.(*circuitbreaker.Metric)
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitIsOpen)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	assert.Equal(t, "backend-service-name-2", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "backend-service-name-2", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())

	time.Sleep(time.Millisecond * 300)
	p.Command = func() error { return nil }
	r, err = p.Run()
	m, _ = r.(*circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name-2", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "backend-service-name-2", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunHandledErrors(t *testing.T) {
	var state circuitbreaker.CircuitState

	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := circuitbreaker.Policy{
		ServiceID:       "backend-service-name-3",
		ThresholdErrors: 1,
		ResetTimeout:    time.Millisecond * 300,
		Errors:          []error{errTest1, errTest2},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	p.Command = func() error { return errTest1 }
	r, err := p.Run()
	m, _ := r.(*circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name-3", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest1)
	assert.Equal(t, "backend-service-name-3", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())

	p.Command = func() error { return errTest2 }
	r, err = p.Run()
	m, _ = r.(*circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name-3", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest2)
	assert.Equal(t, "backend-service-name-3", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunUnhandledError(t *testing.T) {
	var state circuitbreaker.CircuitState

	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := circuitbreaker.Policy{
		ServiceID:       "backend-service-name-4",
		ThresholdErrors: 1,
		ResetTimeout:    time.Millisecond * 300,
		Errors:          []error{errTest1},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	p.Command = func() error { return errTest1 }
	r, err := p.Run()
	m, _ := r.(*circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name-4", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest1)
	assert.Equal(t, "backend-service-name-4", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())

	p.Command = func() error { return errTest2 }
	r, err = p.Run()
	m, _ = r.(*circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	assert.Equal(t, "backend-service-name-4", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest2)
	assert.Equal(t, "backend-service-name-4", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunPolicy(t *testing.T) {
	var state circuitbreaker.CircuitState

	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := circuitbreaker.Policy{
		ServiceID:       "backend-service-name-5",
		ThresholdErrors: 1,
		ResetTimeout:    time.Millisecond * 300,
		Errors:          []error{errTest1, errTest2},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	metric := core.NewMetric()

	p.Command = func() error { return errTest1 }
	err := p.RunPolicy(metric, core.PolicySupplier(p))
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	r := metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ := r.(*circuitbreaker.Metric)

	assert.Equal(t, "backend-service-name-5", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "backend-service-name-5", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}
