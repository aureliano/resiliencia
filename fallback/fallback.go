package fallback

import (
	"errors"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	// No fallback handler provided.
	ErrNoFallBackHandler = errors.New("no fallback handler")

	// No command nor wrapped policy is set.
	ErrCommandRequiredError = errors.New("command nor wrapped policy provided")

	// Unhandled error. It's not in Errors policy field.
	ErrUnhandledError = errors.New("unhandled error")
)

// Policy defines the fallback algorithm execution policy.
type Policy struct {
	// The registered service id.
	ServiceID string

	// Expected erros (not expected errors will make fallback policy fail).
	Errors []error

	// Function to be executed when execution fail.
	FallBackHandler func(err error)

	// Function called before execution.
	BeforeFallBack func(p Policy)

	// Function called after execution.
	AfterFallBack func(p Policy, err error)

	// The command supplier.
	Command core.Command

	// Any policy that will be wrapped by this one.
	Policy core.PolicySupplier
}

// Metric keeps the running state of the fallback.
type Metric struct {
	// The registered service id.
	ID string

	// The execution status (success is non zero).
	Status int

	// When execution started.
	StartedAt time.Time

	// When execution finished.
	FinishedAt time.Time

	// The error (if execution wasn't succeeded)
	Error error
}

// New creates a fallback policy with default values set.
func New(serviceID string) Policy {
	return Policy{ServiceID: serviceID}
}

// Run executes a command supplier or a wrapped policy in a fallback.
//
// Possible error(s): ErrCommandRequired, ErrNoFallBackHandler, ErrorErrUnhandledError.
func (p Policy) Run(metric core.Metric) error {
	if err := validate(p); err != nil {
		return err
	}

	m := Metric{ID: p.ServiceID, StartedAt: time.Now()}
	if p.BeforeFallBack != nil {
		p.BeforeFallBack(p)
	}

	err := execute(p, metric)
	err = pickError(err, metric)

	if p.AfterFallBack != nil {
		p.AfterFallBack(p, err)
	}
	m.FinishedAt = time.Now()

	if !handledError(p, err) || !handledError(p, metric.MetricError()) {
		m.Status = 1
		m.Error = ErrUnhandledError
		metric[reflect.TypeOf(m).String()] = m

		return ErrUnhandledError
	}

	if err != nil {
		p.FallBackHandler(err)
	}
	metric[reflect.TypeOf(m).String()] = m

	return nil
}

// WithCommand encapsulates this policy in a new policy with given command supplier.
func (p Policy) WithCommand(command core.Command) core.PolicySupplier {
	p.Command = command
	return p
}

// WithPolicy encapsulates this policy in a new policy with wrapped policy.
func (p Policy) WithPolicy(policy core.PolicySupplier) core.PolicySupplier {
	p.Policy = policy
	return p
}

func execute(p Policy, metric core.Metric) error {
	if p.Command != nil && p.Policy == nil {
		return p.Command()
	}

	return p.Policy.Run(metric)
}

func handledError(p Policy, err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func validate(p Policy) error {
	switch {
	case p.FallBackHandler == nil:
		return ErrNoFallBackHandler
	case p.Command == nil && p.Policy == nil:
		return ErrCommandRequiredError
	default:
		return nil
	}
}

func pickError(err error, metric core.MetricRecorder) error {
	if err != nil {
		return err
	}

	return metric.MetricError()
}

// ServiceID returns the service id registered to the policy binded to this metric.
func (m Metric) ServiceID() string {
	return m.ID
}

// PolicyDuration returns the policy execution duration.
// In short, finished at less (-) started at.
func (m Metric) PolicyDuration() time.Duration {
	return m.FinishedAt.Sub(m.StartedAt)
}

// Success returns whether the policy execution succeeded or not.
// In short, status is zero and error is nil.
func (m Metric) Success() bool {
	return (m.Status == 0) && (m.Error == nil)
}

// MetricError returns the error that a command supplier or a wrapped policy raised.
func (m Metric) MetricError() error {
	return m.Error
}
