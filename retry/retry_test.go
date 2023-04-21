package retry_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/retry"
	"github.com/stretchr/testify/assert"
)

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
