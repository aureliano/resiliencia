package fallback_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aureliano/resiliencia/fallback"
	"github.com/stretchr/testify/assert"
)

func TestRunValidatePolicyFallBackHandler(t *testing.T) {
	p := fallback.New()
	err := p.Run(context.TODO(), func(ctx context.Context) error { return nil })

	assert.ErrorIs(t, fallback.ErrNoFallBackHandler, err)
}

func TestRunNoFallback(t *testing.T) {
	fallbackCalled := false
	p := fallback.New()
	p.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	p.BeforeFallBack = func(p fallback.Policy) {}
	p.AfterTryFallBack = func(p fallback.Policy, err error) {}
	_ = p.Run(context.TODO(), func(ctx context.Context) error { return nil })

	assert.False(t, fallbackCalled)
}

func TestRunHandleError(t *testing.T) {
	fallbackCalled := false
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := fallback.New()
	p.Errors = []error{errTest1, errTest2}
	p.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	p.BeforeFallBack = func(p fallback.Policy) {}
	p.AfterTryFallBack = func(p fallback.Policy, err error) {}
	err := p.Run(context.TODO(), func(ctx context.Context) error { return errTest2 })

	assert.Nil(t, err)
	assert.True(t, fallbackCalled)
}

func TestRunUnhandleError(t *testing.T) {
	fallbackCalled := false
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")

	p := fallback.New()
	p.Errors = []error{errTest1, errTest2}
	p.FallBackHandler = func(err error) {
		fallbackCalled = true
	}
	p.BeforeFallBack = func(p fallback.Policy) {}
	p.AfterTryFallBack = func(p fallback.Policy, err error) {}
	err := p.Run(context.TODO(), func(ctx context.Context) error { return fmt.Errorf("unknown error") })

	assert.ErrorIs(t, fallback.ErrUnhandledError, err)
	assert.False(t, fallbackCalled)
}
