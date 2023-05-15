package retry_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/retry"
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
	p := retry.New("postForm")
	i := reflect.TypeOf((*core.PolicySupplier)(nil)).Elem()

	assert.True(t, reflect.TypeOf(p).Implements(i))
}

func TestMetricImplementsMetricRecorder(t *testing.T) {
	m := retry.Metric{}
	i := reflect.TypeOf((*core.MetricRecorder)(nil)).Elem()

	assert.True(t, reflect.TypeOf(m).Implements(i))
}

func TestNew(t *testing.T) {
	p := retry.New("postForm")
	assert.Equal(t, "postForm", p.ServiceID)
	assert.Equal(t, 1, p.Tries)
	assert.Equal(t, time.Duration(0), p.Delay)
}

func TestRunValidatePolicyTries(t *testing.T) {
	p := retry.Policy{Tries: 0, Delay: retry.MinDelay, Command: func() error { return nil }}

	metric := core.NewMetric()
	err := p.Run(metric)

	assert.ErrorIs(t, err, retry.ErrTriesValidation)
}

func TestRunValidatePolicyDelay(t *testing.T) {
	p := retry.Policy{Tries: retry.MinTries, Delay: time.Duration(-1), Policy: &mockPolicy{}}

	metric := core.NewMetric()
	err := p.Run(metric)

	assert.ErrorIs(t, err, retry.ErrDelayValidation)
}

func TestRunValidatePolicyCommand(t *testing.T) {
	p := retry.Policy{Tries: retry.MinTries, Delay: retry.MinDelay}

	metric := core.NewMetric()
	err := p.Run(metric)

	assert.ErrorIs(t, err, retry.ErrCommandRequired)
}

func TestRunCommandMaxTriesExceeded(t *testing.T) {
	timesAfter, timesBefore := 0, 0
	errTest := errors.New("any")

	p := retry.New("postForm")
	p.Tries = 3
	p.Errors = []error{errTest}
	p.Delay = time.Millisecond * 10
	p.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	p.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}
	p.Command = func() error { return errTest }

	metric := core.NewMetric()
	err := p.Run(metric)
	i := metric[reflect.TypeOf(retry.Metric{}).String()]
	m, _ := i.(retry.Metric)

	assert.Equal(t, p.Tries, timesBefore)
	assert.Equal(t, p.Tries, timesAfter)
	assert.ErrorIs(t, err, retry.ErrMaxTriesExceeded)
	assert.Equal(t, p.Tries, m.Tries)
	assert.Equal(t, 1, m.Status)
	assert.Equal(t, "postForm", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "postForm", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
	assert.NotNil(t, m.MetricError())

	for _, exec := range m.Executions {
		assert.ErrorIs(t, errTest, exec.Error)
	}
}

func TestRunCommandHandledErrors(t *testing.T) {
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")
	timesAfter, timesBefore := 0, 0

	p := retry.New("postForm")
	p.Tries = 3
	p.Delay = time.Millisecond * 10
	p.Errors = []error{errTest1, errTest2}
	p.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	p.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}

	counter := 0
	p.Command = func() error {
		counter++
		switch {
		case counter == 1:
			return errTest1
		case counter == 2:
			return errTest2
		default:
			return nil
		}
	}

	metric := core.NewMetric()
	err := p.Run(metric)
	i := metric[reflect.TypeOf(retry.Metric{}).String()]
	m, _ := i.(retry.Metric)

	assert.Equal(t, 3, timesBefore)
	assert.Equal(t, 3, timesAfter)
	assert.Nil(t, err)

	assert.Equal(t, p.Tries, m.Tries)
	assert.Equal(t, 0, m.Status)
	assert.Equal(t, "postForm", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "postForm", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
	assert.Nil(t, m.MetricError())

	assert.ErrorIs(t, errTest1, m.Executions[0].Error)
	assert.ErrorIs(t, errTest2, m.Executions[1].Error)
	assert.Nil(t, m.Executions[2].Error)
}

func TestRunCommandUnhandledError(t *testing.T) {
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")
	errTest3 := errors.New("error test 3")
	timesAfter, timesBefore := 0, 0

	p := retry.New("postForm")
	p.Tries = 5
	p.Delay = time.Millisecond * 10
	p.Errors = []error{errTest1, errTest2}
	p.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	p.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}

	counter := 0
	p.Command = func() error {
		counter++
		switch {
		case counter == 1:
			return errTest1
		case counter == 2:
			return errTest2
		case counter == 3:
			return errTest3
		default:
			return nil
		}
	}

	metric := core.NewMetric()
	err := p.Run(metric)
	i := metric[reflect.TypeOf(retry.Metric{}).String()]
	m, _ := i.(retry.Metric)

	assert.Equal(t, 3, timesBefore)
	assert.Equal(t, 3, timesAfter)
	assert.ErrorIs(t, err, retry.ErrUnhandledError)

	assert.Equal(t, 3, m.Tries)
	assert.Equal(t, 1, m.Status)
	assert.Equal(t, "postForm", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "postForm", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
	assert.NotNil(t, m.MetricError())

	assert.ErrorIs(t, errTest1, m.Executions[0].Error)
	assert.ErrorIs(t, errTest2, m.Executions[1].Error)
	assert.ErrorIs(t, errTest3, m.Executions[2].Error)
}

func TestRunCommand(t *testing.T) {
	timesAfter, timesBefore := 0, 0

	p := retry.New("postForm")
	p.Tries = 3
	p.Delay = time.Millisecond * 10
	p.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	p.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}
	p.Command = func() error { return nil }

	metric := core.NewMetric()
	err := p.Run(metric)
	i := metric[reflect.TypeOf(retry.Metric{}).String()]
	m, _ := i.(retry.Metric)

	assert.Equal(t, 1, timesBefore)
	assert.Equal(t, 1, timesAfter)
	assert.Nil(t, err)

	assert.Equal(t, 1, m.Tries)
	assert.Equal(t, 0, m.Status)
	assert.Equal(t, "postForm", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "postForm", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*50)
	assert.True(t, m.Success())
	assert.Nil(t, m.MetricError())
}

func TestRunPolicy(t *testing.T) {
	timesAfter, timesBefore := 0, 0

	policy := new(mockPolicy)
	policy.On("Run").Return(nil)
	mockMetric := Metric{
		ID:         "dummy-service",
		Status:     0,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	retryPolicy := retry.New("remote-service")
	retryPolicy.Tries = 3
	retryPolicy.Delay = time.Millisecond * 30
	retryPolicy.Policy = policy
	retryPolicy.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	retryPolicy.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}
	metric := core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric

	err := retryPolicy.Run(metric)
	assert.Nil(t, err)
	assert.Equal(t, 1, timesBefore)
	assert.Equal(t, 1, timesAfter)

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
	assert.Nil(t, childMetric.MetricError())

	r = metric[reflect.TypeOf(retry.Metric{}).String()]
	retryMetric, _ := r.(retry.Metric)

	assert.Equal(t, "remote-service", retryMetric.ID)
	assert.Equal(t, 0, retryMetric.Status)
	assert.Less(t, retryMetric.StartedAt, retryMetric.FinishedAt)
	assert.Len(t, retryMetric.Executions, 1)
	assert.Nil(t, retryMetric.Error)
	assert.Nil(t, retryMetric.Executions[0].Error)
	assert.Equal(t, "remote-service", retryMetric.ServiceID())
	assert.True(t, retryMetric.Success())
	assert.Nil(t, retryMetric.MetricError())
}

func TestRunPolicyUnhandledError(t *testing.T) {
	errTest := errors.New("error test")
	timesAfter, timesBefore := 0, 0

	policy := new(mockPolicy)
	policy.On("Run").Return(errTest)
	mockMetric := Metric{
		ID:         "dummy-service",
		Status:     1,
		Error:      fmt.Errorf("any"),
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	retryPolicy := retry.New("remote-service")
	retryPolicy.Tries = 3
	retryPolicy.Delay = time.Millisecond * 10
	retryPolicy.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	retryPolicy.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}
	retryPolicy.Policy = policy
	metric := core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric

	err := retryPolicy.Run(metric)
	assert.ErrorIs(t, err, retry.ErrUnhandledError)
	assert.Equal(t, 1, timesBefore)
	assert.Equal(t, 1, timesAfter)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.NotNil(t, childMetric.MetricError())
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Millisecond*150)
	assert.Greater(t, time.Millisecond*151, childMetric.PolicyDuration())
	assert.False(t, childMetric.Success())

	r = metric[reflect.TypeOf(retry.Metric{}).String()]
	retryMetric, _ := r.(retry.Metric)

	assert.Equal(t, "remote-service", retryMetric.ID)
	assert.Equal(t, 1, retryMetric.Status)
	assert.Less(t, retryMetric.StartedAt, retryMetric.FinishedAt)
	assert.Len(t, retryMetric.Executions, 1)
	assert.ErrorIs(t, retryMetric.Error, retry.ErrUnhandledError)
	assert.ErrorIs(t, retryMetric.Executions[0].Error, errTest)
	assert.Equal(t, "remote-service", retryMetric.ServiceID())
	assert.False(t, retryMetric.Success())
	assert.NotNil(t, retryMetric.MetricError())
}

func TestRunPolicyMaxTriesExceeded(t *testing.T) {
	errTest := errors.New("error test")
	timesAfter, timesBefore := 0, 0

	policy := new(mockPolicy)
	policy.On("Run").Return(errTest)
	mockMetric := Metric{
		ID:         "dummy-service",
		Status:     1,
		Error:      fmt.Errorf("any"),
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}

	retryPolicy := retry.New("remote-service")
	retryPolicy.Tries = 3
	retryPolicy.Delay = time.Millisecond * 10
	retryPolicy.Errors = []error{errTest}
	retryPolicy.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	retryPolicy.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}
	retryPolicy.Policy = policy
	metric := core.NewMetric()
	metric[reflect.TypeOf(mockMetric).String()] = mockMetric

	err := retryPolicy.Run(metric)
	assert.ErrorIs(t, err, retry.ErrMaxTriesExceeded)
	assert.Equal(t, retryPolicy.Tries, timesBefore)
	assert.Equal(t, retryPolicy.Tries, timesAfter)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.NotNil(t, childMetric.MetricError())
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Millisecond*150)
	assert.Greater(t, time.Millisecond*151, childMetric.PolicyDuration())
	assert.False(t, childMetric.Success())

	r = metric[reflect.TypeOf(retry.Metric{}).String()]
	retryMetric, _ := r.(retry.Metric)

	assert.Equal(t, "remote-service", retryMetric.ID)
	assert.Equal(t, 1, retryMetric.Status)
	assert.Less(t, retryMetric.StartedAt, retryMetric.FinishedAt)
	assert.Len(t, retryMetric.Executions, 3)
	assert.ErrorIs(t, retryMetric.Error, retry.ErrMaxTriesExceeded)
	for i := 0; i < retryPolicy.Tries; i++ {
		assert.ErrorIs(t, retryMetric.Executions[i].Error, errTest)
	}
	assert.Equal(t, "remote-service", retryMetric.ServiceID())
	assert.False(t, retryMetric.Success())
	assert.NotNil(t, retryMetric.MetricError())
}

func TestWithCommand(t *testing.T) {
	p := retry.New("id")
	assert.Nil(t, p.Command)

	np := p.WithCommand(func() error { return nil })
	p, _ = np.(retry.Policy)
	assert.NotNil(t, p.Command)
}

func TestWithPolicy(t *testing.T) {
	p := retry.New("id")
	assert.Nil(t, p.Policy)

	np := p.WithPolicy(&mockPolicy{})
	p, _ = np.(retry.Policy)
	assert.Equal(t, &mockPolicy{}, p.Policy)
}
