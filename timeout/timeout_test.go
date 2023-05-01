package timeout_test

import (
	"errors"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/timeout"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	p := timeout.New()
	assert.Equal(t, time.Duration(0), p.Timeout)
}

func TestRunValidatePolicyTimeout(t *testing.T) {
	p := timeout.New()
	p.Timeout = -1
	_, err := p.Run(func() error { return nil })

	assert.ErrorIs(t, timeout.ErrTimeoutError, err)
}

func TestRun(t *testing.T) {
	p := timeout.New()
	p.Timeout = time.Second * 4
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	m, err := p.Run(func() error { return nil })

	assert.Nil(t, err)

	assert.Equal(t, "", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunWithUnknownError(t *testing.T) {
	errTest := errors.New("err test")
	p := timeout.New()
	p.Timeout = time.Second * 4
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	m, err := p.Run(func() error { return errTest })

	assert.Nil(t, err)

	assert.Equal(t, "", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest)
	assert.Equal(t, "", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.True(t, m.Success())
}

func TestRunTimeout(t *testing.T) {
	p := timeout.New()
	p.Timeout = time.Millisecond * 500
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	m, err := p.Run(func() error {
		time.Sleep(time.Millisecond * 550)
		return nil
	})

	assert.ErrorIs(t, timeout.ErrTimeoutError, err)

	assert.Equal(t, "", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Nil(t, m.Error)
	assert.Equal(t, "", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}
