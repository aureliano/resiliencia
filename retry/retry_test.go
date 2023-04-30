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

func TestRunValidatePolicyTries(t *testing.T) {
	p := retry.Policy{Tries: 0, Delay: time.Duration(100)}
	_, err := p.Run(context.TODO(), func(ctx context.Context) error { return nil })

	assert.ErrorIs(t, retry.ErrTriesError, err)
}

func TestRunValidatePolicyDelay(t *testing.T) {
	p := retry.Policy{Tries: 10, Delay: time.Duration(-1)}
	_, err := p.Run(context.TODO(), func(ctx context.Context) error { return nil })

	assert.ErrorIs(t, retry.ErrDelayError, err)
}

func TestRunMaxTriesExceeded(t *testing.T) {
	timesAfter, timesBefore := 0, 0
	errTest := errors.New("any")

	p := retry.New()
	p.Tries = 3
	p.Errors = []error{errTest}
	p.Delay = time.Millisecond * 10
	p.BeforeTry = func(p retry.Policy, try int) {
		timesBefore++
	}
	p.AfterTry = func(p retry.Policy, try int, err error) {
		timesAfter++
	}

	ctx := context.TODO()
	m, e := p.Run(ctx, func(ctx context.Context) error {
		return errTest
	})

	assert.Equal(t, p.Tries, timesBefore)
	assert.Equal(t, p.Tries, timesAfter)
	assert.ErrorIs(t, e, retry.ErrExceededTries)

	assert.Equal(t, p.Tries, m.Tries)
	assert.Equal(t, 1, m.Status)
	assert.Equal(t, "", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "", m.ServiceID())
	assert.Greater(t, time.Microsecond*100, m.PolicyDuration())
	assert.False(t, m.Success())
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
	m, e := p.Run(ctx, func(ctx context.Context) error {
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

	assert.Equal(t, p.Tries, m.Tries)
	assert.Equal(t, 0, m.Status)
	assert.Equal(t, "", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "", m.ServiceID())
	assert.Greater(t, time.Microsecond*100, m.PolicyDuration())
	assert.True(t, m.Success())
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
	m, e := p.Run(ctx, func(ctx context.Context) error {
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

	assert.Equal(t, 3, m.Tries)
	assert.Equal(t, 1, m.Status)
	assert.Equal(t, "", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "", m.ServiceID())
	assert.Greater(t, time.Microsecond*100, m.PolicyDuration())
	assert.False(t, m.Success())
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
	m, e := p.Run(ctx, func(ctx context.Context) error {
		return nil
	})

	assert.Equal(t, 1, timesBefore)
	assert.Equal(t, 1, timesAfter)
	assert.Nil(t, e)

	assert.Equal(t, 1, m.Tries)
	assert.Equal(t, 0, m.Status)
	assert.Equal(t, "", m.ID)
	assert.Less(t, m.StartedAt, m.FinishedAt)
	assert.Equal(t, "", m.ServiceID())
	assert.Greater(t, time.Microsecond*100, m.PolicyDuration())
	assert.True(t, m.Success())
}
