package timeout_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/timeout"
	"github.com/stretchr/testify/assert"
)

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
	_, err := p.Run()

	assert.ErrorIs(t, timeout.ErrTimeoutError, err)
}

func TestRun(t *testing.T) {
	p := timeout.New("remote-service")
	p.Timeout = time.Second * 4
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	p.Command = func() error { return nil }

	r, err := p.Run()
	m, _ := r.(*timeout.Metric)

	assert.Nil(t, err)

	assert.Equal(t, "remote-service", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "remote-service", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunWithUnknownError(t *testing.T) {
	errTest := errors.New("err test")
	p := timeout.New("remote-service")
	p.Timeout = time.Second * 4
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	p.Command = func() error { return errTest }

	r, err := p.Run()
	m, _ := r.(*timeout.Metric)

	assert.Nil(t, err)

	assert.Equal(t, "remote-service", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest)
	assert.Equal(t, "remote-service", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunTimeout(t *testing.T) {
	p := timeout.New("remote-service")
	p.Timeout = time.Millisecond * 500
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	p.Command = func() error {
		time.Sleep(time.Millisecond * 550)
		return nil
	}

	r, err := p.Run()
	m, _ := r.(*timeout.Metric)

	assert.ErrorIs(t, timeout.ErrTimeoutError, err)

	assert.Equal(t, "remote-service", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "remote-service", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}

func TestRunPolicy(t *testing.T) {
	p := timeout.New("remote-service")
	p.Timeout = time.Millisecond * 100
	p.Command = func() error { return nil }
	metric := core.NewMetric()

	err := p.RunPolicy(metric, core.PolicySupplier(p))
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(timeout.Metric{}).String()]
	m, _ := r.(*timeout.Metric)

	assert.Equal(t, "remote-service", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "remote-service", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}
