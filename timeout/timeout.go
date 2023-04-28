package timeout

import (
	"context"
	"errors"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrTimeoutError = errors.New("timeout must be >= 0")
)

type Policy struct {
	Timeout       time.Duration
	BeforeTimeout func(p Policy)
	AfterTimeout  func(p Policy, err error)
}

func New() Policy {
	return Policy{
		Timeout: 0,
	}
}

func (p Policy) Run(ctx context.Context, cmd core.Command) error {
	if err := validatePolicy(p); err != nil {
		return err
	}

	if p.BeforeTimeout != nil {
		p.BeforeTimeout(p)
	}

	var err, cmdErr error
	c := make(chan string)
	go func() {
		c <- "start"
		cmdErr = cmd(ctx)
		c <- "done"
	}()

waiting:
	for {
		select {
		case str := <-c:
			if str == "done" {
				break waiting
			}
		case <-time.After(p.Timeout):
			err = ErrTimeoutError
			break waiting
		}
	}

	if p.AfterTimeout != nil {
		p.AfterTimeout(p, cmdErr)
	}

	return err
}

func validatePolicy(p Policy) error {
	if p.Timeout < 0 {
		return ErrTimeoutError
	}

	return nil
}
