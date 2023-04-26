package circuitbreaker

import (
	"context"
	"errors"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrCircuitBreakError = errors.New("...")
)

type Policy struct {
	ThresholdErrors      int
	ResetTimeout         time.Duration
	Errors               []error
	BeforeCircuitBreaker func(p Policy)
	AfterCircuitBreaker  func(p Policy, err error)
	OnOpenCircuit        func(p Policy, err error)
	OnHalfOpenCircuit    func(p Policy)
	OnClosedCircuit      func(p Policy)
}

func New() Policy {
	return Policy{
		ThresholdErrors: 1,
		ResetTimeout:    0,
	}
}

func (p Policy) Run(ctx context.Context, cmd core.Command) error { return nil }
