package retry

import (
	"context"
	"errors"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrExceededTries = errors.New("max tries reached")
)

type Policy struct {
	Tries     int
	Delay     time.Duration
	BeforeTry func(p Policy, try int)
	AfterTry  func(p Policy, try int, err error)
}

func New() Policy {
	return Policy{
		Tries: 1,
		Delay: 0,
	}
}

func (p Policy) Run(ctx context.Context, cmd core.Command) error {
	done := false

	for i := 0; i < p.Tries; i++ {
		turn := i + 1
		if p.BeforeTry != nil {
			p.BeforeTry(p, turn)
		}

		err := cmd(ctx)

		if p.AfterTry != nil {
			p.AfterTry(p, turn, err)
		}

		if err == nil {
			done = true
			break
		}

		time.Sleep(p.Delay)
	}

	if !done {
		return ErrExceededTries
	}

	return nil
}
