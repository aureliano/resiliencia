package fallback

import (
	"context"
	"errors"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrNoFallBackHandler = errors.New("no fallback handler")
	ErrUnhandledError    = errors.New("unhandled error")
)

type Policy struct {
	Errors           []error
	FallBackHandler  func(err error)
	BeforeFallBack   func(p Policy)
	AfterTryFallBack func(p Policy, err error)
}

func New() Policy {
	return Policy{}
}

func (p Policy) Run(ctx context.Context, cmd core.Command) error {
	if err := p.validate(); err != nil {
		return err
	}

	if p.BeforeFallBack != nil {
		p.BeforeFallBack(p)
	}

	err := cmd(ctx)

	if p.AfterTryFallBack != nil {
		p.AfterTryFallBack(p, err)
	}

	if err != nil && !p.handledError(err) {
		return ErrUnhandledError
	}

	if err != nil {
		p.FallBackHandler(err)
	}

	return nil
}

func (p Policy) handledError(err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func (p Policy) validate() error {
	if p.FallBackHandler == nil {
		return ErrNoFallBackHandler
	}

	return nil
}
