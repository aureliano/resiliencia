package core

import (
	"context"
	"errors"
	"time"
)

type Command func() error

type PolicySupplier interface {
	Run(ctx context.Context, cmd Command) (MetricRecorder, error)
	handledError(err error) bool
	validate() error
}

type MetricRecorder interface {
	ServiceID() string
	PolicyDuration() time.Duration
	Success() bool
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
