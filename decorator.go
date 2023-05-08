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
	Retry          *retry.Policy
	Timeout        *timeout.Policy
	Fallback       *fallback.Policy
	CircuitBreaker *circuitbreaker.Policy
}

type Decorator interface {
	WithRetry(policy retry.Policy) Decorator
	WithTimeout(policy timeout.Policy) Decorator
	WithFallback(policy fallback.Policy) Decorator
	WithCircuitBreaker(policy circuitbreaker.Policy) Decorator
	Execute() (core.Metric, error)
}

func (d Decoration) WithRetry(policy retry.Policy) Decorator {
	d.Retry = &policy
	return d
}

func (d Decoration) WithTimeout(policy timeout.Policy) Decorator {
	d.Timeout = &policy
	return d
}

func (d Decoration) WithFallback(policy fallback.Policy) Decorator {
	d.Fallback = &policy
	return d
}

func (d Decoration) WithCircuitBreaker(policy circuitbreaker.Policy) Decorator {
	d.CircuitBreaker = &policy
	return d
}

func (d Decoration) Execute() (core.Metric, error) {
	if err := validateDecorator(d); err != nil {
		return nil, err
	}

	metric := core.NewMetric()
	var err error

	switch {
	case d.Fallback != nil:
		prepareFallback(&d)
		err = d.Fallback.Run(metric)
	case d.CircuitBreaker != nil:
		prepareCircuitBreaker(&d)
		err = d.CircuitBreaker.Run(metric)
	case d.Retry != nil:
		prepareRetry(&d)
		err = d.Retry.Run(metric)
	case d.Timeout != nil:
		d.Timeout.Command = d.Supplier
		err = d.Timeout.Run(metric)
	}

	return metric, err
}

func prepareFallback(d *Decoration) {
	switch {
	case fallbackCompleteChain(d):
		d.Timeout.Command = d.Supplier
		d.Retry.Policy = d.Timeout
		d.CircuitBreaker.Policy = d.Retry
		d.Fallback.Policy = d.CircuitBreaker
	case d.CircuitBreaker != nil && d.Retry != nil && d.Timeout == nil:
		d.Retry.Command = d.Supplier
		d.CircuitBreaker.Policy = d.Retry
		d.Fallback.Policy = d.CircuitBreaker
	case d.CircuitBreaker != nil && d.Retry == nil && d.Timeout == nil:
		d.CircuitBreaker.Command = d.Supplier
		d.Fallback.Policy = d.CircuitBreaker
	case fallbackOnly(d):
		d.Fallback.Command = d.Supplier
	}
}

func fallbackCompleteChain(d *Decoration) bool {
	return d.CircuitBreaker != nil && d.Retry != nil && d.Timeout != nil
}

func fallbackOnly(d *Decoration) bool {
	return d.CircuitBreaker == nil && d.Retry == nil && d.Timeout == nil
}

func prepareCircuitBreaker(d *Decoration) {
	switch {
	case d.Retry != nil && d.Timeout != nil:
		d.Timeout.Command = d.Supplier
		d.Retry.Policy = d.Timeout
		d.CircuitBreaker.Policy = d.Retry
	case d.Retry != nil && d.Timeout == nil:
		d.Retry.Command = d.Supplier
		d.CircuitBreaker.Policy = d.Retry
	case d.Retry == nil && d.Timeout == nil:
		d.CircuitBreaker.Command = d.Supplier
	}
}

func prepareRetry(d *Decoration) {
	if d.Timeout != nil {
		d.Timeout.Command = d.Supplier
		d.Retry.Policy = d.Timeout
	} else {
		d.Retry.Command = d.Supplier
	}
}

func validateDecorator(d Decoration) error {
	switch {
	case !anyPolicyProvided(d):
		return ErrPolicyRequired
	case d.Supplier == nil:
		return ErrSupplierRequired
	case anyWrappedPolicyWithCommand(d):
		return ErrWrappedPolicyWithCommand
	case anyWrappedPolicyWithNestedPolicy(d):
		return ErrWrappedPolicyWithNestedPolicy
	default:
		return nil
	}
}

func anyPolicyProvided(d Decoration) bool {
	return d.CircuitBreaker != nil || d.Fallback != nil || d.Retry != nil || d.Timeout != nil
}

func anyWrappedPolicyWithCommand(d Decoration) bool {
	cbCmd := d.CircuitBreaker != nil && d.CircuitBreaker.Command != nil
	fbCmd := d.Fallback != nil && d.Fallback.Command != nil
	rtCmd := d.Retry != nil && d.Retry.Command != nil
	tmCmd := d.Timeout != nil && d.Timeout.Command != nil

	return cbCmd || fbCmd || rtCmd || tmCmd
}

func anyWrappedPolicyWithNestedPolicy(d Decoration) bool {
	cbCmd := d.CircuitBreaker != nil && d.CircuitBreaker.Policy != nil
	fbCmd := d.Fallback != nil && d.Fallback.Policy != nil
	rtCmd := d.Retry != nil && d.Retry.Policy != nil
	tmCmd := d.Timeout != nil && d.Timeout.Policy != nil

	return cbCmd || fbCmd || rtCmd || tmCmd
}
