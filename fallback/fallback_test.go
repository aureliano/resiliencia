package fallback_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/fallback"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPolicy struct{ mock.Mock }

func (p *mockPolicy) Run() (core.MetricRecorder, error) {
	args := p.Called()
	metric := args.Get(0)

	if metric != nil {
		return metric.(core.MetricRecorder), args.Error(1)
	}

	return nil, args.Error(1)
}

func (p *mockPolicy) RunPolicy(metric core.Metric, supplier core.PolicySupplier) error {
	args := p.Called(metric, supplier)
	return args.Error(1)
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
	p := fallback.New("service-id")
	i := reflect.TypeOf((*core.PolicySupplier)(nil)).Elem()

	assert.True(t, reflect.TypeOf(p).Implements(i))
}

func TestMetricImplementsMetricRecorder(t *testing.T) {
	m := fallback.Metric{}
	i := reflect.TypeOf((*core.MetricRecorder)(nil)).Elem()

	assert.True(t, reflect.TypeOf(m).Implements(i))
}

func TestNew(t *testing.T) {
	p := fallback.New("service-id")
	assert.Equal(t, "service-id", p.ServiceID)
}

func TestRunValidatePolicyFallBackHandler(t *testing.T) {
	p := fallback.New("service-id")
	p.Command = func() error { return nil }
	_, err := p.Run()

	assert.ErrorIs(t, err, fallback.ErrNoFallBackHandler)
}

func TestRunValidatePolicyCommand(t *testing.T) {
	p := fallback.New("service-id")
	p.FallBackHandler = func(err error) {}

	_, err := p.Run()

	assert.ErrorIs(t, err, fallback.ErrCommandRequiredError)
}

func TestRunNoFallback(t *testing.T) {
	fallbackCalled := false
	p := fallback.New("service-id")
	p.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	p.BeforeFallBack = func(p fallback.Policy) {}
	p.AfterFallBack = func(p fallback.Policy, err error) {}
	p.Command = func() error { return nil }

	r, _ := p.Run()
	m, _ := r.(fallback.Metric)

	assert.False(t, fallbackCalled)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "service-id", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunHandleError(t *testing.T) {
	fallbackCalled := false
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := fallback.New("service-id")
	p.Errors = []error{errTest1, errTest2}
	p.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	p.BeforeFallBack = func(p fallback.Policy) {}
	p.AfterFallBack = func(p fallback.Policy, err error) {}
	p.Command = func() error { return errTest2 }

	r, err := p.Run()
	m, _ := r.(fallback.Metric)

	assert.Nil(t, err)
	assert.True(t, fallbackCalled)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "service-id", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunUnhandledError(t *testing.T) {
	fallbackCalled := false
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")
	errTest3 := errors.New("error test 3")

	p := fallback.New("service-id")
	p.Errors = []error{errTest1, errTest2}
	p.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	p.BeforeFallBack = func(p fallback.Policy) {}
	p.AfterFallBack = func(p fallback.Policy, err error) {}
	p.Command = func() error { return errTest3 }

	r, err := p.Run()
	m, _ := r.(fallback.Metric)

	assert.ErrorIs(t, fallback.ErrUnhandledError, err)
	assert.False(t, fallbackCalled)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, fallback.ErrUnhandledError)
	assert.Equal(t, "service-id", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}

func TestRunPolicyUnhandledError(t *testing.T) {
	fallbackCalled := false
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")
	errTest3 := errors.New("error test 3")

	policy := new(mockPolicy)

	policy.On("Run").Return(Metric{
		ID:         "dummy-service",
		Status:     1,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
		Error:      errTest3,
	}, errTest3)

	fallbackPolicy := fallback.New("service-id")
	fallbackPolicy.Errors = []error{errTest1, errTest2}
	fallbackPolicy.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	fallbackPolicy.BeforeFallBack = func(p fallback.Policy) {}
	fallbackPolicy.AfterFallBack = func(p fallback.Policy, err error) {}
	fallbackPolicy.Command = func() error { return errTest3 }

	metric := core.NewMetric()
	err := fallbackPolicy.RunPolicy(metric, policy)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.ErrorIs(t, fallback.ErrUnhandledError, err)
	assert.False(t, fallbackCalled)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.ErrorIs(t, childMetric.Error, errTest3)
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, childMetric.Success())

	r = metric[reflect.TypeOf(fallback.Metric{}).String()]
	fallbackMetric, _ := r.(fallback.Metric)

	assert.Equal(t, "service-id", fallbackMetric.ID)
	assert.Equal(t, 1, fallbackMetric.Status)
	assert.Less(t, fallbackMetric.StartedAt, fallbackMetric.FinishedAt)
	assert.ErrorIs(t, fallbackMetric.Error, fallback.ErrUnhandledError)
	assert.Equal(t, "service-id", fallbackMetric.ServiceID())
	assert.Greater(t, fallbackMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, fallbackMetric.Success())
}

func TestRunPolicyHandledError(t *testing.T) {
	fallbackCalled := false
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	policy := new(mockPolicy)

	policy.On("Run").Return(Metric{
		ID:         "dummy-service",
		Status:     1,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
		Error:      errTest2,
	}, errTest2)

	fallbackPolicy := fallback.New("service-id")
	fallbackPolicy.Errors = []error{errTest1, errTest2}
	fallbackPolicy.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	fallbackPolicy.BeforeFallBack = func(p fallback.Policy) {}
	fallbackPolicy.AfterFallBack = func(p fallback.Policy, err error) {}
	fallbackPolicy.Command = func() error { return nil }

	metric := core.NewMetric()
	err := fallbackPolicy.RunPolicy(metric, policy)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Nil(t, err)
	assert.True(t, fallbackCalled)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 1, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.ErrorIs(t, childMetric.Error, errTest2)
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, childMetric.Success())

	r = metric[reflect.TypeOf(fallback.Metric{}).String()]
	fallbackMetric, _ := r.(fallback.Metric)

	assert.Equal(t, "service-id", fallbackMetric.ID)
	assert.Equal(t, 0, fallbackMetric.Status)
	assert.Less(t, fallbackMetric.StartedAt, fallbackMetric.FinishedAt)
	assert.Nil(t, fallbackMetric.Error)
	assert.Equal(t, "service-id", fallbackMetric.ServiceID())
	assert.Greater(t, fallbackMetric.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, fallbackMetric.Success())
}

func TestRunPolicyNoFallback(t *testing.T) {
	fallbackCalled := false

	policy := new(mockPolicy)

	policy.On("Run").Return(Metric{
		ID:         "dummy-service",
		Status:     0,
		StartedAt:  time.Now().Add(time.Millisecond * -150),
		FinishedAt: time.Now(),
	}, nil)

	fallbackPolicy := fallback.New("service-id")
	fallbackPolicy.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	fallbackPolicy.BeforeFallBack = func(p fallback.Policy) {}
	fallbackPolicy.AfterFallBack = func(p fallback.Policy, err error) {}
	fallbackPolicy.Command = func() error { return nil }

	metric := core.NewMetric()
	err := fallbackPolicy.RunPolicy(metric, policy)

	r := metric[reflect.TypeOf(Metric{}).String()]
	childMetric, _ := r.(Metric)

	assert.Nil(t, err)
	assert.False(t, fallbackCalled)

	assert.Equal(t, "dummy-service", childMetric.ID)
	assert.Equal(t, 0, childMetric.Status)
	assert.Less(t, childMetric.StartedAt, childMetric.FinishedAt)
	assert.Nil(t, childMetric.Error)
	assert.Equal(t, "dummy-service", childMetric.ServiceID())
	assert.Greater(t, childMetric.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, childMetric.Success())

	r = metric[reflect.TypeOf(fallback.Metric{}).String()]
	fallbackMetric, _ := r.(fallback.Metric)

	assert.Equal(t, "service-id", fallbackMetric.ID)
	assert.Equal(t, 0, fallbackMetric.Status)
	assert.Less(t, fallbackMetric.StartedAt, fallbackMetric.FinishedAt)
	assert.Nil(t, fallbackMetric.Error)
	assert.Equal(t, "service-id", fallbackMetric.ServiceID())
	assert.Greater(t, fallbackMetric.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, fallbackMetric.Success())
}
