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

func (d Decoration) WithRetry(policy retry.Policy) Decorator {
	d.Retry = policy
	return d
}

func (d Decoration) WithTimeout(policy timeout.Policy) Decorator {
	d.Timeout = policy
	return d
}

func (d Decoration) WithFallback(policy []fallback.Policy) Decorator {
	d.Fallback = policy
	return d
}

func (d Decoration) WithCircuitBreaker(policy circuitbreaker.Policy) Decorator {
	d.CircuitBreaker = policy
	return d
}

/*func (d Decoration) Execute() {
	d.CircuitBreaker.RunPolicy(nil, d.Retry.RunPolicy(nil, d.Timeout.RunPolicy(nil, d.Supplier)))
}*/
