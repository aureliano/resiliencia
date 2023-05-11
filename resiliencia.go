package resiliencia

import (
	"errors"

	"github.com/aureliano/resiliencia/core"
)

var (
	// Policy is required.
	ErrPolicyRequired = errors.New("at least one policy is required")

	// Coomand supplier is required.
	ErrSupplierRequired = errors.New("command supplier is required")

	// Policy can't have wrapped policy and command supplier set.
	ErrWrappedPolicyWithCommand = errors.New("wrapped policy with command")

	// Wrapped or chained policy cannot have policy nor command suplier set.
	ErrWrappedPolicyWithNestedPolicy = errors.New("wrapped policy with nested policy")
)

// Decorate creates a policies decorator given a command supplier.
//
// Returns a decorator with command.
func Decorate(command core.Command) Decorator {
	return Decoration{Supplier: command}
}

// Chain creates a policies chain of responsibility.
//
// Returns a policies chain.
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
