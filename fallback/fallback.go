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
	if err := validatePolicy(p); err != nil {
		return err
	}

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

	if err != nil {
		p.FallBackHandler(err)
	}

	return nil
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

func validatePolicy(p Policy) error {
	if p.FallBackHandler == nil {
		return ErrNoFallBackHandler
	}

	return nil
}
