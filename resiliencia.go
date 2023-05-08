package resiliencia

import (
	"errors"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrPolicyRequired                = errors.New("at least one policy is required")
	ErrSupplierRequired              = errors.New("supplier is required")
	ErrWrappedPolicyWithCommand      = errors.New("wrapped policy with command")
	ErrWrappedPolicyWithNestedPolicy = errors.New("wrapped policy with nested policy")
)

func Decorate(command core.Command) Decorator {
	return Decoration{Supplier: command}
}
