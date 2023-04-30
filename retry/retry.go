package retry

import (
	"context"
	"errors"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrExceededTries  = errors.New("max tries reached")
	ErrUnhandledError = errors.New("unhandled error")
	ErrDelayError     = errors.New("delay must be >= 0")
	ErrTriesError     = errors.New("tries must be > 0")
)

type Policy struct {
	Tries     int
	Delay     time.Duration
	Errors    []error
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
	if err := validatePolicy(p); err != nil {
		return err
	}

	for i := 0; i < p.Tries; i++ {
		turn := i + 1
		if p.BeforeTry != nil {
			p.BeforeTry(p, turn)
		}

		err := cmd(ctx)

		if p.AfterTry != nil {
			p.AfterTry(p, turn, err)
		}

		if err != nil && !p.handledError(err) {
			return ErrUnhandledError
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

func (p Policy) handledError(err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func validatePolicy(p Policy) error {
	switch {
	case p.Delay < 0:
		return ErrDelayError
	case p.Tries <= 0:
		return ErrTriesError
	default:
		return nil
	}
}
