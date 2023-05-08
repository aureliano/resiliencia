package resiliencia

import (
	"errors"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrPolicyRequired                = errors.New("at least one policy is required")
	ErrSupplierRequired              = errors.New("command supplier is required")
	ErrWrappedPolicyWithCommand      = errors.New("wrapped policy with command")
	ErrWrappedPolicyWithNestedPolicy = errors.New("wrapped policy with nested policy")
)

func Decorate(command core.Command) Decorator {
	return Decoration{Supplier: command}
}

func Chain(policies ...core.PolicySupplier) Chainer {
	return ChainOfResponsibility{Policies: removeNull(policies)}
}

func removeNull(array []core.PolicySupplier) []core.PolicySupplier {
	newArray := make([]core.PolicySupplier, 0, len(array))

	for _, item := range array {
		if item != nil {
			newArray = append(newArray, item)
		}
	}

	return newArray
}
