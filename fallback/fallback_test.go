package fallback_test

import (
	"errors"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/fallback"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	p := fallback.New("service-id")
	assert.Equal(t, "service-id", p.ServiceID)
}

func TestRunValidatePolicyFallBackHandler(t *testing.T) {
	p := fallback.New("service-id")
	_, err := p.Run(func() error { return nil })

	assert.ErrorIs(t, err, fallback.ErrNoFallBackHandler)
}

func TestRunNoFallback(t *testing.T) {
	fallbackCalled := false
	p := fallback.New("service-id")
	p.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	p.BeforeFallBack = func(p fallback.Policy) {}
	p.AfterFallBack = func(p fallback.Policy, err error) {}
	m, _ := p.Run(func() error { return nil })

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
	m, err := p.Run(func() error { return errTest2 })

	assert.Nil(t, err)
	assert.True(t, fallbackCalled)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest2)
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
	m, err := p.Run(func() error { return errTest3 })

	assert.ErrorIs(t, fallback.ErrUnhandledError, err)
	assert.False(t, fallbackCalled)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 1, m.Status)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.ErrorIs(t, m.Error, errTest3)
	assert.Equal(t, "service-id", m.ServiceID())
	assert.Greater(t, m.PolicyDuration(), time.Nanosecond*100)
	assert.False(t, m.Success())
}
