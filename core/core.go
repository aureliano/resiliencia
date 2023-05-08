package core

import (
	"errors"
	"time"
)

type Command func() error

type PolicySupplier interface {
	Run(metric Metric) error
	WithCommand(command Command) PolicySupplier
	WithPolicy(policy PolicySupplier) PolicySupplier
}

type MetricRecorder interface {
	ServiceID() string
	PolicyDuration() time.Duration
	Success() bool
}

type Metric map[string]MetricRecorder

func NewMetric() Metric {
	return make(map[string]MetricRecorder)
}

func (Metric) ServiceID() string {
	return "policy-chain"
}

func (m Metric) PolicyDuration() time.Duration {
	duration := time.Duration(0)
	for _, v := range m {
		duration += v.PolicyDuration()
	}

	return duration
}

func (m Metric) Success() bool {
	for _, v := range m {
		if !v.Success() {
			return false
		}
	}

	return true
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
