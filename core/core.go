package core

import (
	"errors"
	"time"
)

// Command is the type that Policies use as a supplier. Indeed, it is just a
// pointer to an anonymous function.
type Command func() error

// PolicySupplier is the interface that all Policies must implement. Its contract
// defines some obligations that every policy must follow.
type PolicySupplier interface {
	// Run is the method's definition of a running policy.
	Run(metric Metric) error

	// WithCommand is the method's definition of a policy encapsulation with the command supplier.
	WithCommand(command Command) PolicySupplier

	// WithPolicy is the method's definition of a policy encapsulation with another policy.
	WithPolicy(policy PolicySupplier) PolicySupplier
}

// MetricRecorder is the interface that all Metrics must implement. It defines some behaviors that
// metrics are expected to be.
type MetricRecorder interface {
	// ServiceID returns the service id.
	ServiceID() string

	// PolicyDuration returns the policy execution duration.
	PolicyDuration() time.Duration

	// Success returns whether the policy execution succeeded or not.
	Success() bool

	MetricError() error
}

// Metric is the base metric recorder type. This is the one which is passed through the life
// cycle of an execution chain.
type Metric map[string]MetricRecorder

// NewMetric makes and returns a new metric.
func NewMetric() Metric {
	return make(map[string]MetricRecorder)
}

// ServiceID returns the service id registered to the policy binded to this metric.
func (Metric) ServiceID() string {
	return "policy-chain"
}

// PolicyDuration returns the policy execution duration.
// As this metric is an array of chained metrics, duration calculation is done by iterating
// through all metrics in the chain, summing the return of each call to PolicyDuration.
//
// Returns the amount of time spent for all policies to execute.
func (m Metric) PolicyDuration() time.Duration {
	duration := time.Duration(0)
	for _, v := range m {
		duration += v.PolicyDuration()
	}

	return duration
}

// Success returns whether the policy execution succeeded or not.
// As this metric is an array of chained metrics, verification of success is done by iterating
// through all metrics in the chain and confirming that all calls to the Success method returned true.
//
// Return: Whether all metrics returned succesfuly.
func (m Metric) Success() bool {
	for _, v := range m {
		if !v.Success() {
			return false
		}
	}

	return true
}

// MetricError returns the first error occurred in the policy chain.
//
// Return: the first found error.
func (m Metric) MetricError() error {
	for _, v := range m {
		if !v.Success() {
			return v.MetricError()
		}
	}

	return nil
}

// ErrorInErrors Verifies that an error is a slice of expected errors.
//
// Return: whether err is in expectedErrors.
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
