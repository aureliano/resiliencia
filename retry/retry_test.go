package retry_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/retry"
	"github.com/stretchr/testify/assert"
)

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
	p := retry.Policy{Tries: 0, Delay: time.Duration(100), Command: func() error { return nil }}
	_, err := p.Run()

	assert.ErrorIs(t, err, retry.ErrTriesError)
}

func TestRunValidatePolicyDelay(t *testing.T) {
	p := retry.Policy{Tries: 10, Delay: time.Duration(-1), Command: func() error { return nil }}
	_, err := p.Run()

	assert.ErrorIs(t, err, retry.ErrDelayError)
}

func TestRunMaxTriesExceeded(t *testing.T) {
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

	r, e := p.Run()
	m, _ := r.(*retry.Metric)

	assert.Equal(t, p.Tries, timesBefore)
	assert.Equal(t, p.Tries, timesAfter)
	assert.ErrorIs(t, e, retry.ErrExceededTries)

	assert.Equal(t, p.Tries, m.Tries)
	assert.Equal(t, 1, m.Status)
	assert.Equal(t, "postForm", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "postForm", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())

	for _, exec := range m.Executions {
		assert.ErrorIs(t, errTest, exec.Error)
	}
}

func TestRunHandledErrors(t *testing.T) {
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

	r, e := p.Run()
	m, _ := r.(*retry.Metric)

	assert.Equal(t, 3, timesBefore)
	assert.Equal(t, 3, timesAfter)
	assert.Nil(t, e)

	assert.Equal(t, p.Tries, m.Tries)
	assert.Equal(t, 0, m.Status)
	assert.Equal(t, "postForm", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "postForm", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())

	assert.ErrorIs(t, errTest1, m.Executions[0].Error)
	assert.ErrorIs(t, errTest2, m.Executions[1].Error)
	assert.Nil(t, m.Executions[2].Error)
}

func TestRunUnhandledError(t *testing.T) {
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

	r, e := p.Run()
	m, _ := r.(*retry.Metric)

	assert.Equal(t, 3, timesBefore)
	assert.Equal(t, 3, timesAfter)
	assert.ErrorIs(t, e, retry.ErrUnhandledError)

	assert.Equal(t, 3, m.Tries)
	assert.Equal(t, 1, m.Status)
	assert.Equal(t, "postForm", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "postForm", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())

	assert.ErrorIs(t, errTest1, m.Executions[0].Error)
	assert.ErrorIs(t, errTest2, m.Executions[1].Error)
	assert.ErrorIs(t, errTest3, m.Executions[2].Error)
}

func TestRun(t *testing.T) {
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

	r, e := p.Run()
	m, _ := r.(*retry.Metric)

	assert.Equal(t, 1, timesBefore)
	assert.Equal(t, 1, timesAfter)
	assert.Nil(t, e)

	assert.Equal(t, 1, m.Tries)
	assert.Equal(t, 0, m.Status)
	assert.Equal(t, "postForm", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "postForm", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunPolicy(t *testing.T) {
	p := retry.New("postForm")
	p.Command = func() error { return nil }
	metric := core.NewMetric()

	err := p.RunPolicy(metric, core.PolicySupplier(p))
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(retry.Metric{}).String()]
	m, _ := r.(*retry.Metric)

	assert.Equal(t, 1, m.Tries)
	assert.Equal(t, 0, m.Status)
	assert.Equal(t, "postForm", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "postForm", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}
