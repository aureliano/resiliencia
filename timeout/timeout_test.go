package timeout_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/timeout"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPolicy struct{ mock.Mock }

func (p *mockPolicy) Run(_ core.Metric) error {
	args := p.Called()
	return args.Error(0)
}

func (p *mockPolicy) WithCommand(_ core.Command) core.PolicySupplier {
	args := p.Called()
	return args.Get(0).(core.PolicySupplier)
}

func (p *mockPolicy) WithPolicy(_ core.PolicySupplier) core.PolicySupplier {
	args := p.Called()
	return args.Get(0).(core.PolicySupplier)
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

func (m Metric) MetricError() error {
	return m.Error
}

func TestPolicyImplementsPolicySupplier(t *testing.T) {
	p := timeout.New("remote-service")
	i := reflect.TypeOf((*core.PolicySupplier)(nil)).Elem()

	assert.True(t, reflect.TypeOf(p).Implements(i))
}

func TestMetricImplementsMetricRecorder(t *testing.T) {
	m := timeout.Metric{}
	i := reflect.TypeOf((*core.MetricRecorder)(nil)).Elem()

	assert.True(t, reflect.TypeOf(m).Implements(i))
}

func TestNew(t *testing.T) {
	p := timeout.New("remote-service")
	assert.Equal(t, "remote-service", p.ServiceID)
	assert.Equal(t, time.Duration(0), p.Timeout)
}

func TestRunValidatePolicyTimeout(t *testing.T) {
	p := timeout.New("remote-service")
	p.Timeout = -1
	p.Command = func() error { return nil }

	metric := core.NewMetric()
	err := p.Run(metric)

	assert.ErrorIs(t, err, timeout.ErrTimeoutValidation)

	p.Command = nil
	p.Policy = &mockPolicy{}

	metric = core.NewMetric()
	err = p.Run(metric)

	assert.ErrorIs(t, err, timeout.ErrTimeoutValidation)
}

func TestRunValidatePolicyCommand(t *testing.T) {
	p := timeout.New("remote-service")
	p.Timeout = timeout.MinTimeout

	metric := core.NewMetric()
	err := p.Run(metric)

	assert.ErrorIs(t, err, timeout.ErrCommandRequired)
}

func TestRunCommand(t *testing.T) {
	p := timeout.New("remote-service")
	p.Timeout = time.Second * 4
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	p.Command = func() error { return nil }

	metric := core.NewMetric()
	err := p.Run(metric)
	i := metric[reflect.TypeOf(timeout.Metric{}).String()]
	m, _ := i.(timeout.Metric)

	assert.Nil(t, err)

	assert.Equal(t, "remote-service", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.MetricError())
	assert.Equal(t, "remote-service", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunCommandWithUnknownError(t *testing.T) {
	errTest := errors.New("err test")
	p := timeout.New("remote-service")
	p.Timeout = time.Second * 4
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	p.Command = func() error { return errTest }

	metric := core.NewMetric()
	err := p.Run(metric)
	i := metric[reflect.TypeOf(timeout.Metric{}).String()]
	m, _ := i.(timeout.Metric)

	assert.Nil(t, err)

	assert.Equal(t, "remote-service", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.MetricError(), errTest)
	assert.Equal(t, "remote-service", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}

func TestRunCommandTimeout(t *testing.T) {
	p := timeout.New("remote-service")
	p.Timeout = time.Millisecond * 50
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	p.Command = func() error {
		time.Sleep(time.Millisecond * 55)
		return nil
	}

	metric := core.NewMetric()
	err := p.Run(metric)
	i := metric[reflect.TypeOf(timeout.Metric{}).String()]
	m, _ := i.(timeout.Metric)

	assert.ErrorIs(t, timeout.ErrExecutionTimedOut, err)

	assert.Equal(t, "remote-service", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, timeout.ErrExecutionTimedOut, m.MetricError())
	assert.Equal(t, "remote-service", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}

func TestRunPolicySuccess(t *testing.T) {
	policy := new(mockPolicy)
	policy.On("Run").Return(nil)
	mockMetric := Metric{
		ID:         "dummy-service",
		Status:     0,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	timeoutPolicy := timeout.New("remote-service")
	timeoutPolicy.Timeout = time.Millisecond * 100
	timeoutPolicy.Policy = policy
	metric := core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric

	err := timeoutPolicy.Run(metric)
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 0, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.Nil(t, childMetric.Error)
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Millisecond*150)
	assert.Greater(t, time.Millisecond*151, childMetric.PolicyDuration())
	assert.True(t, childMetric.Success())

	r = metric[reflect.TypeOf(timeout.Metric{}).String()]
	timeoutMetric, _ := r.(timeout.Metric)

	assert.Equal(t, "remote-service", timeoutMetric.ID)
	assert.Equal(t, 0, timeoutMetric.Status)
	assert.Less(t, timeoutMetric.StartedAt, timeoutMetric.FinishedAt)
	assert.Nil(t, timeoutMetric.Error)
	assert.Equal(t, "remote-service", timeoutMetric.ServiceID())
	assert.True(t, timeoutMetric.Success())
}

func TestRunPolicyChildFail(t *testing.T) {
	errTest := fmt.Errorf("child error")

	policy := new(mockPolicy)
	policy.On("Run").Return(errTest)
	mockMetric := Metric{
		ID:         "dummy-service",
		Status:     1,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	timeoutPolicy := timeout.New("remote-service")
	timeoutPolicy.Timeout = time.Millisecond * 10
	timeoutPolicy.Policy = policy
	metric := core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric

	err := timeoutPolicy.Run(metric)
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.Nil(t, childMetric.Error)
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Millisecond*150)
	assert.Greater(t, time.Millisecond*151, childMetric.PolicyDuration())
	assert.False(t, childMetric.Success())

	r = metric[reflect.TypeOf(timeout.Metric{}).String()]
	timeoutMetric, _ := r.(timeout.Metric)

	assert.Equal(t, "remote-service", timeoutMetric.ID)
	assert.Equal(t, 1, timeoutMetric.Status)
	assert.Less(t, timeoutMetric.StartedAt, timeoutMetric.FinishedAt)
	assert.ErrorIs(t, timeoutMetric.Error, errTest)
	assert.Equal(t, "remote-service", timeoutMetric.ServiceID())
	assert.False(t, timeoutMetric.Success())
}

func TestRunPolicyTimeout(t *testing.T) {
	policy := new(mockPolicy)
	policy.On("Run").After(time.Millisecond * 15).Return(nil)

	timeoutPolicy := timeout.New("remote-service")
	timeoutPolicy.Timeout = time.Millisecond * 10
	timeoutPolicy.Policy = policy
	metric := core.NewMetric()

	err := timeoutPolicy.Run(metric)
	assert.ErrorIs(t, err, timeout.ErrExecutionTimedOut)

	r := metric[reflect.TypeOf(Metric{}).String()]
	assert.Nil(t, r)

	r = metric[reflect.TypeOf(timeout.Metric{}).String()]
	timeoutMetric, _ := r.(timeout.Metric)

	assert.Equal(t, "remote-service", timeoutMetric.ID)
	assert.Equal(t, 1, timeoutMetric.Status)
	assert.Less(t, timeoutMetric.StartedAt, timeoutMetric.FinishedAt)
	assert.ErrorIs(t, timeoutMetric.Error, timeout.ErrExecutionTimedOut)
	assert.Equal(t, "remote-service", timeoutMetric.ServiceID())
	assert.False(t, timeoutMetric.Success())
}

func TestWithCommand(t *testing.T) {
	p := timeout.New("id")
	assert.Nil(t, p.Command)

	np := p.WithCommand(func() error { return nil })
	p, _ = np.(timeout.Policy)
	assert.NotNil(t, p.Command)
}

func TestWithPolicy(t *testing.T) {
	p := timeout.New("id")
	assert.Nil(t, p.Policy)

	np := p.WithPolicy(&mockPolicy{})
	p, _ = np.(timeout.Policy)
	assert.Equal(t, &mockPolicy{}, p.Policy)
}
