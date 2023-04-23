package timeout_test

import (
	"context"
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
	err := p.Run(context.TODO(), func(ctx context.Context) error { return nil })

	assert.ErrorIs(t, timeout.ErrTimeoutError, err)
}

func TestRun(t *testing.T) {
	p := timeout.New()
	p.Timeout = time.Second * 4
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	err := p.Run(context.TODO(), func(ctx context.Context) error {
		return nil
	})

	assert.Nil(t, err)
}

func TestRunTimeout(t *testing.T) {
	p := timeout.New()
	p.Timeout = time.Millisecond * 500
	p.BeforeTimeout = func(p timeout.Policy) {}
	p.AfterTimeout = func(p timeout.Policy, err error) {}
	err := p.Run(context.TODO(), func(ctx context.Context) error {
		time.Sleep(time.Millisecond * 550)
		return nil
	})

	assert.ErrorIs(t, timeout.ErrTimeoutError, err)
}
