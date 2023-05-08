package resiliencia

import (
	"github.com/aureliano/resiliencia/core"
)

type ChainOfResponsibility struct {
	Policies []core.PolicySupplier
}

type Chainer interface {
	Execute(command core.Command) (core.Metric, error)
}

func (c ChainOfResponsibility) Execute(command core.Command) (core.Metric, error) {
	if err := validateChain(c, command); err != nil {
		return nil, err
	}

	lindex := len(c.Policies) - 1
	for i := lindex; i >= 0; i-- {
		if i == lindex {
			c.Policies[i] = c.Policies[i].WithCommand(command)
			c.Policies[i] = c.Policies[i].WithPolicy(nil)
		} else {
			c.Policies[i] = c.Policies[i].WithCommand(nil)
			c.Policies[i] = c.Policies[i].WithPolicy(c.Policies[i+1])
		}
	}

	metric := core.NewMetric()
	err := c.Policies[0].Run(metric)

	return metric, err
}

func validateChain(c ChainOfResponsibility, cmd core.Command) error {
	switch {
	case len(c.Policies) == 0:
		return ErrPolicyRequired
	case cmd == nil:
		return ErrSupplierRequired
	default:
		return nil
	}
}
