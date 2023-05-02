package resiliencia

import (
	"github.com/aureliano/resiliencia/circuitbreaker"
	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/fallback"
	"github.com/aureliano/resiliencia/retry"
	"github.com/aureliano/resiliencia/timeout"
)

type Decoration struct {
	Supplier       core.Command
	Retry          retry.Policy
	Timeout        timeout.Policy
	Fallback       []fallback.Policy
	CircuitBreaker circuitbreaker.Policy
}

type Decorator interface {
	WithRetry(policy retry.Policy) Decorator
	WithTimeout(policy timeout.Policy) Decorator
	WithFallback(policy []fallback.Policy) Decorator
	WithCircuitBreaker(policy circuitbreaker.Policy) Decorator
}

func Decorate(command core.Command) Decorator {
	return Decoration{Supplier: command}
}

func (p Decoration) WithRetry(policy retry.Policy) Decorator {
	p.Retry = policy
	return p
}

func (p Decoration) WithTimeout(policy timeout.Policy) Decorator {
	p.Timeout = policy
	return p
}

func (p Decoration) WithFallback(policy []fallback.Policy) Decorator {
	p.Fallback = policy
	return p
}

func (p Decoration) WithCircuitBreaker(policy circuitbreaker.Policy) Decorator {
	p.CircuitBreaker = policy
	return p
}
