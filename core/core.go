package core

import (
	"context"
	"errors"
)

type Command func(ctx context.Context) error

type Suplier interface {
	Run(ctx context.Context, cmd Command) error
}

func ErrorInErrors(expectedErrors []error, err error) bool {
	if len(expectedErrors) == 0 {
		return false
	}

	for _, expectedError := range expectedErrors {
		if errors.Is(expectedError, err) {
			return true
		}
	}

	return false
}
