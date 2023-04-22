package fallback

import (
	"context"
	"errors"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrUnhandledError = errors.New("unhandled error")
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
	if p.BeforeFallBack != nil {
		p.BeforeFallBack(p)
	}

	err := cmd(ctx)

	if p.AfterTryFallBack != nil {
		p.AfterTryFallBack(p, err)
	}

	if err != nil && !handledError(p, err) {
		return ErrUnhandledError
	}

	p.FallBackHandler(err)

	return err
}

func handledError(p Policy, err error) bool {
	if p.Errors == nil || len(p.Errors) == 0 {
		return true
	}

	for _, expectedError := range p.Errors {
		if errors.Is(expectedError, err) {
			return true
		}
	}

	return false
}
