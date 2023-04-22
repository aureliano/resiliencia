package retry_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/retry"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	p := retry.New()
	assert.Equal(t, 1, p.Tries)
	assert.Equal(t, time.Duration(0), p.Delay)
}

func TestRunMaxTriesExceeded(t *testing.T) {
	timesAfter, timesBefore := 0, 0

	p := retry.New()
	p.Tries = 3
	p.Delay = time.Millisecond * 10
	p.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	p.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}

	ctx := context.TODO()
	e := p.Run(ctx, func(ctx context.Context) error {
		return fmt.Errorf("any error")
	})

	assert.Equal(t, p.Tries, timesBefore)
	assert.Equal(t, p.Tries, timesAfter)
	assert.ErrorIs(t, e, retry.ErrExceededTries)
}

func TestRunHandledErrors(t *testing.T) {
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")
	timesAfter, timesBefore := 0, 0

	p := retry.New()
	p.Tries = 3
	p.Delay = time.Millisecond * 10
	p.Errors = []error{errTest1, errTest2}
	p.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	p.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}

	ctx := context.TODO()
	counter := 0
	e := p.Run(ctx, func(ctx context.Context) error {
		counter++
		switch {
		case counter == 1:
			return errTest1
		case counter == 2:
			return errTest2
		default:
			return nil
		}
	})

	assert.Equal(t, 3, timesBefore)
	assert.Equal(t, 3, timesAfter)
	assert.Nil(t, e)
}

func TestRunUnhandledError(t *testing.T) {
	errTest1 := errors.New("error test 1")
	errTest2 := errors.New("error test 2")
	timesAfter, timesBefore := 0, 0

	p := retry.New()
	p.Tries = 5
	p.Delay = time.Millisecond * 10
	p.Errors = []error{errTest1, errTest2}
	p.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	p.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}

	ctx := context.TODO()
	counter := 0
	e := p.Run(ctx, func(ctx context.Context) error {
		counter++
		switch {
		case counter == 1:
			return errTest1
		case counter == 2:
			return errTest2
		case counter == 3:
			return fmt.Errorf("unknown error")
		default:
			return nil
		}
	})

	assert.Equal(t, 3, timesBefore)
	assert.Equal(t, 3, timesAfter)
	assert.ErrorIs(t, retry.ErrUnhandledError, e)
}

func TestRun(t *testing.T) {
	timesAfter, timesBefore := 0, 0

	p := retry.New()
	p.Tries = 3
	p.Delay = time.Millisecond * 10
	p.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	p.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}

	ctx := context.TODO()
	e := p.Run(ctx, func(ctx context.Context) error {
		return nil
	})

	assert.Equal(t, 1, timesBefore)
	assert.Equal(t, 1, timesAfter)
	assert.Nil(t, e)
}
