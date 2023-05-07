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
	"github.com/stretchr/testify/mock"
)

type mockPolicy struct{ mock.Mock }

func (p *mockPolicy) Run(_ core.Metric) error {
	args := p.Called()
	return args.Error(0)
}

type Metric struct {
	ID         string
	Status     int
	StartedAt  time.Time
	FinishedAt time.Time
	Error      error
}

func (m Metric) ServiceID() string {
	return m.ID
}

func (m Metric) PolicyDuration() time.Duration {
	return m.FinishedAt.Sub(m.StartedAt)
}

func (m Metric) Success() bool {
	return m.Status == 0
}

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
	metric := core.NewMetric()
	err = p.Run(metric)
	assert.Nil(t, err)
	state, err = circuitbreaker.State(p)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	p.Command = func() error { return fmt.Errorf("any") }
	metric = core.NewMetric()
	err = p.Run(metric)
	assert.Nil(t, err)
	state, err = circuitbreaker.State(p)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	time.Sleep(time.Millisecond * 50)
	state, err = circuitbreaker.State(p)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.HalfOpenState, state)

	p.Command = func() error { return nil }
	metric = core.NewMetric()
	err = p.Run(metric)
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
	p := circuitbreaker.Policy{
		ThresholdErrors: 0,
		ResetTimeout:    circuitbreaker.MinResetTimeout,
		Command:         func() error { return nil },
	}
	err := p.Run(core.NewMetric())

	assert.ErrorIs(t, err, circuitbreaker.ErrThresholdError)
}

func TestRunValidatePolicyResetTimeout(t *testing.T) {
	p := circuitbreaker.Policy{
		ThresholdErrors: circuitbreaker.MinThresholdErrors,
		ResetTimeout:    time.Millisecond * 1,
		Policy:          &mockPolicy{},
	}

	err := p.Run(core.NewMetric())

	assert.ErrorIs(t, err, circuitbreaker.ErrResetTimeoutError)
}

func TestRunValidatePolicyCommand(t *testing.T) {
	p := circuitbreaker.Policy{
		ThresholdErrors: circuitbreaker.MinThresholdErrors,
		ResetTimeout:    circuitbreaker.MinResetTimeout,
	}

	err := p.Run(core.NewMetric())

	assert.ErrorIs(t, err, circuitbreaker.ErrCommandRequiredError)
}

func TestRunCommandCircuitIsOpen(t *testing.T) {
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

	r := core.NewMetric()
	err := p.Run(r)
	i := r[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ := i.(circuitbreaker.Metric)
	assert.Nil(t, err)

	assert.Equal(t, "backend-service-name-1", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest)
	assert.EqualValues(t, circuitbreaker.OpenState, m.State)
	assert.Equal(t, 1, m.ErrorCount)
	assert.Equal(t, "backend-service-name-1", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())

	p.Command = func() error { return nil }
	r = core.NewMetric()
	err = p.Run(r)
	i = r[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ = i.(circuitbreaker.Metric)
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitIsOpen)

	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, circuitbreaker.ErrCircuitIsOpen)
	assert.EqualValues(t, circuitbreaker.OpenState, m.State)
	assert.Equal(t, 1, m.ErrorCount)
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}

func TestRunCommandCircuitHalfOpenSetToClosed(t *testing.T) {
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
	r := core.NewMetric()
	err := p.Run(r)
	i := r[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ := i.(circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	assert.Equal(t, "backend-service-name-2", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest)
	assert.Equal(t, "backend-service-name-2", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())

	p.Command = func() error { return nil }
	r = core.NewMetric()
	err = p.Run(r)
	i = r[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ = i.(circuitbreaker.Metric)
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitIsOpen)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	assert.Equal(t, "backend-service-name-2", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, circuitbreaker.ErrCircuitIsOpen)
	assert.Equal(t, "backend-service-name-2", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())

	time.Sleep(time.Millisecond * 300)
	p.Command = func() error { return nil }
	r = core.NewMetric()
	err = p.Run(r)
	i = r[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ = i.(circuitbreaker.Metric)
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

func TestRunCommandHandledErrors(t *testing.T) {
	var state circuitbreaker.CircuitState

	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := circuitbreaker.Policy{
		ServiceID:       "backend-service-name-3",
		ThresholdErrors: 3,
		ResetTimeout:    time.Millisecond * 300,
		Errors:          []error{errTest1, errTest2},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	p.Command = func() error { return errTest1 }
	r := core.NewMetric()
	err := p.Run(r)
	i := r[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ := i.(circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name-3", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest1)
	assert.Equal(t, "backend-service-name-3", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())

	p.Command = func() error { return errTest2 }
	r = core.NewMetric()
	err = p.Run(r)
	i = r[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ = i.(circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name-3", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest2)
	assert.Equal(t, "backend-service-name-3", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}

func TestRunCommandUnhandledError(t *testing.T) {
	var state circuitbreaker.CircuitState

	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := circuitbreaker.Policy{
		ServiceID:       "backend-service-name-4",
		ThresholdErrors: 3,
		ResetTimeout:    time.Millisecond * 300,
		Errors:          []error{errTest1},
		AfterCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {
			state = status.State
		},
	}

	p.Command = func() error { return errTest1 }
	r := core.NewMetric()
	err := p.Run(r)
	i := r[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ := i.(circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	assert.Equal(t, "backend-service-name-4", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest1)
	assert.Equal(t, "backend-service-name-4", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())

	p.Command = func() error { return errTest2 }
	r = core.NewMetric()
	err = p.Run(r)
	i = r[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ = i.(circuitbreaker.Metric)
	assert.Nil(t, err)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	assert.Equal(t, "backend-service-name-4", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest2)
	assert.Equal(t, "backend-service-name-4", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}

func TestRunPolicyCircuitIsOpen(t *testing.T) {
	errTest := errors.New("err test")

	policy := new(mockPolicy)
	policy.On("Run").Return(errTest)
	mockMetric := Metric{
		ID:         "dummy-service",
		Status:     1,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	cbPolicy := circuitbreaker.Policy{
		ServiceID:            "backend-service-name-5",
		ThresholdErrors:      1,
		ResetTimeout:         time.Second * 1,
		BeforeCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnOpenCircuit:        func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
		OnHalfOpenCircuit:    func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnClosedCircuit:      func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		AfterCircuitBreaker:  func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
	}
	cbPolicy.Policy = policy

	metric := core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric
	err := cbPolicy.Run(metric)
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, childMetric.Success())

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "backend-service-name-5", cbMetric.ID)
	assert.Equal(t, 1, cbMetric.Status)
	assert.Less(t, cbMetric.StartedAt, cbMetric.FinishedAt)
	assert.ErrorIs(t, cbMetric.Error, errTest)
	assert.EqualValues(t, circuitbreaker.OpenState, cbMetric.State)
	assert.Equal(t, 1, cbMetric.ErrorCount)
	assert.Equal(t, "backend-service-name-5", cbMetric.ServiceID())
	assert.Greater(t, cbMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, cbMetric.Success())

	cbPolicy.Policy = policy

	metric = core.NewMetric()
	err = cbPolicy.Run(metric)
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitIsOpen)

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ = r.(circuitbreaker.Metric)

	assert.Equal(t, 1, cbMetric.Status)
	assert.Less(t, cbMetric.StartedAt, cbMetric.FinishedAt)
	assert.ErrorIs(t, cbMetric.Error, circuitbreaker.ErrCircuitIsOpen)
	assert.EqualValues(t, circuitbreaker.OpenState, cbMetric.State)
	assert.Equal(t, 1, cbMetric.ErrorCount)
	assert.Greater(t, cbMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, cbMetric.Success())
}

func TestRunPolicyCircuitHalfOpenSetToClosed(t *testing.T) {
	errTest := errors.New("err test")

	policy := new(mockPolicy)
	policy.On("Run").Return(errTest)
	mockMetric := Metric{
		ID:         "dummy-service",
		Status:     1,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	cbPolicy := circuitbreaker.Policy{
		ServiceID:            "backend-service-name-6",
		ThresholdErrors:      1,
		ResetTimeout:         time.Millisecond * 300,
		BeforeCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnOpenCircuit:        func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
		OnHalfOpenCircuit:    func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnClosedCircuit:      func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		AfterCircuitBreaker:  func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
		Policy:               policy,
	}

	metric := core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric
	err := cbPolicy.Run(metric)
	assert.Nil(t, err)

	state, _ := circuitbreaker.State(cbPolicy)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, childMetric.Success())

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "backend-service-name-6", cbMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, cbMetric.StartedAt, cbMetric.FinishedAt)
	assert.ErrorIs(t, cbMetric.Error, errTest)
	assert.Equal(t, "backend-service-name-6", cbMetric.ServiceID())
	assert.Greater(t, cbMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, cbMetric.Success())

	metric = core.NewMetric()
	err = cbPolicy.Run(metric)
	assert.ErrorIs(t, err, circuitbreaker.ErrCircuitIsOpen)

	state, _ = circuitbreaker.State(cbPolicy)
	assert.EqualValues(t, circuitbreaker.OpenState, state)

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ = r.(circuitbreaker.Metric)

	assert.Equal(t, "backend-service-name-6", cbMetric.ID)
	assert.Equal(t, 1, cbMetric.Status)
	assert.Less(t, cbMetric.StartedAt, cbMetric.FinishedAt)
	assert.ErrorIs(t, cbMetric.Error, circuitbreaker.ErrCircuitIsOpen)
	assert.Equal(t, "backend-service-name-6", cbMetric.ServiceID())
	assert.Greater(t, cbMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, cbMetric.Success())

	time.Sleep(time.Millisecond * 300)

	state, _ = circuitbreaker.State(cbPolicy)
	assert.EqualValues(t, circuitbreaker.HalfOpenState, state)

	policy = new(mockPolicy)
	policy.On("Run").Return(nil)
	mockMetric = Metric{
		ID:         "dummy-service",
		Status:     0,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	metric = core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric
	cbPolicy.Policy = policy
	err = cbPolicy.Run(metric)
	assert.Nil(t, err)

	state, _ = circuitbreaker.State(cbPolicy)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ = r.(circuitbreaker.Metric)

	assert.Equal(t, "backend-service-name-6", cbMetric.ID)
	assert.Equal(t, 0, cbMetric.Status)
	assert.Less(t, cbMetric.StartedAt, cbMetric.FinishedAt)
	assert.Nil(t, cbMetric.Error)
	assert.Equal(t, "backend-service-name-6", cbMetric.ServiceID())
	assert.Greater(t, cbMetric.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, cbMetric.Success())
}

func TestRunPolicyHandledErrors(t *testing.T) {
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	policy := new(mockPolicy)
	policy.On("Run").Return(errTest1)
	mockMetric := Metric{
		ID:         "dummy-service",
		Status:     1,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	cbPolicy := circuitbreaker.Policy{
		ServiceID:            "backend-service-name-7",
		ThresholdErrors:      3,
		ResetTimeout:         time.Millisecond * 300,
		BeforeCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnOpenCircuit:        func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
		OnHalfOpenCircuit:    func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnClosedCircuit:      func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		AfterCircuitBreaker:  func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
		Errors:               []error{errTest1, errTest2},
		Policy:               policy,
	}

	metric := core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric
	err := cbPolicy.Run(metric)
	assert.Nil(t, err)

	state, _ := circuitbreaker.State(cbPolicy)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, childMetric.Success())

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "backend-service-name-7", cbMetric.ID)
	assert.Equal(t, 1, cbMetric.Status)
	assert.Less(t, cbMetric.StartedAt, cbMetric.FinishedAt)
	assert.ErrorIs(t, cbMetric.Error, errTest1)
	assert.Equal(t, "backend-service-name-7", cbMetric.ServiceID())
	assert.Greater(t, cbMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, cbMetric.Success())

	state, _ = circuitbreaker.State(cbPolicy)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	policy = new(mockPolicy)
	policy.On("Run").Return(errTest2)
	mockMetric = Metric{
		ID:         "dummy-service",
		Status:     2,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	metric = core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric
	cbPolicy.Policy = policy
	err = cbPolicy.Run(metric)
	assert.Nil(t, err)

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ = r.(circuitbreaker.Metric)

	assert.Equal(t, "backend-service-name-7", cbMetric.ID)
	assert.Equal(t, 1, cbMetric.Status)
	assert.Less(t, cbMetric.StartedAt, cbMetric.FinishedAt)
	assert.ErrorIs(t, cbMetric.Error, errTest2)
	assert.Equal(t, "backend-service-name-7", cbMetric.ServiceID())
	assert.Greater(t, cbMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, cbMetric.Success())

	state, _ = circuitbreaker.State(cbPolicy)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)
}

func TestRunPolicyUnhandledError(t *testing.T) {
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	policy := new(mockPolicy)
	policy.On("Run").Return(errTest1)
	mockMetric := Metric{
		ID:         "dummy-service",
		Status:     1,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	cbPolicy := circuitbreaker.Policy{
		ServiceID:            "backend-service-name-8",
		ThresholdErrors:      3,
		ResetTimeout:         time.Millisecond * 300,
		BeforeCircuitBreaker: func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnOpenCircuit:        func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
		OnHalfOpenCircuit:    func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		OnClosedCircuit:      func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {},
		AfterCircuitBreaker:  func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {},
		Errors:               []error{errTest1},
		Policy:               policy,
	}

	metric := core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric
	err := cbPolicy.Run(metric)
	assert.Nil(t, err)

	state, _ := circuitbreaker.State(cbPolicy)
	assert.EqualValues(t, circuitbreaker.ClosedState, state)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, childMetric.Success())

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "backend-service-name-8", cbMetric.ID)
	assert.Equal(t, 1, cbMetric.Status)
	assert.Less(t, cbMetric.StartedAt, cbMetric.FinishedAt)
	assert.Equal(t, "backend-service-name-8", cbMetric.ServiceID())
	assert.Greater(t, cbMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, cbMetric.Success())

	policy = new(mockPolicy)
	policy.On("Run").Return(errTest2)
	mockMetric = Metric{
		ID:         "dummy-service",
		Status:     2,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	metric = core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric
	cbPolicy.Policy = policy
	err = cbPolicy.Run(metric)
	assert.Nil(t, err)

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ = r.(circuitbreaker.Metric)

	assert.Equal(t, "backend-service-name-8", cbMetric.ID)
	assert.Equal(t, 1, cbMetric.Status)
	assert.Less(t, cbMetric.StartedAt, cbMetric.FinishedAt)
	assert.ErrorIs(t, cbMetric.Error, errTest2)
	assert.Equal(t, "backend-service-name-8", cbMetric.ServiceID())
	assert.Greater(t, cbMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, cbMetric.Success())

	state, _ = circuitbreaker.State(cbPolicy)
	assert.EqualValues(t, circuitbreaker.OpenState, state)
}
