package retry

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	// Policy delay is less than minimum required.
	ErrDelayValidation = fmt.Errorf("delay must be >= %d", MinDelay)

	// Policy tries is less than minimum required.
	ErrTriesValidation = fmt.Errorf("tries must be >= %d", MinTries)

	// Number of executions has reached the limit.
	ErrMaxTriesExceeded = errors.New("max tries reached")

	// Unhandled error. It's not in Errors policy field.
	ErrUnhandledError = errors.New("unhandled error")

	// No command nor wrapped policy is set.
	ErrCommandRequired = errors.New("command nor wrapped policy provided")
)

// Policy defines the retry algorithm execution policy.
type Policy struct {
	// The registered service id.
	ServiceID string

	// Number of executions to be tried until fail.
	Tries int

	// Delay between each execution.
	Delay time.Duration

	// Expected erros (not expected errors will abort execution).
	Errors []error

	// Function called before each execution.
	BeforeTry func(p Policy, try int)

	// Function called after each execution.
	AfterTry func(p Policy, try int, err error)

	// The command supplier.
	Command core.Command

	// Any policy that will be wrapped by this one.
	Policy core.PolicySupplier
}

// Metric keeps the running state of the retry.
type Metric struct {
	// The registered service id.
	ID string

	// Number of executions.
	Tries int

	// The execution status (success is non zero).
	Status int

	// When execution started.
	StartedAt time.Time

	// When execution finished.
	FinishedAt time.Time

	// The error (if execution wasn't succeeded)
	Error error

	// Execution metrics.
	Executions []struct {
		// Iteration id. Starts from one (1).
		Iteration int

		// When execution started.
		StartedAt time.Time

		// When execution finished.
		FinishedAt time.Time

		// Execution duration.
		Duration time.Duration

		// The error (if execution wasn't succeeded)
		Error error
	}
}

const (
	// Minimum expected to be set on Delay field of a retry policy.
	MinDelay = 0

	// Minimum expected to be set on Tries field of a retry policy.
	MinTries = 1
)

// New creates a retry policy with default values set.
func New(serviceID string) Policy {
	return Policy{
		ServiceID: serviceID,
		Tries:     1,
		Delay:     0,
	}
}

// Run executes a command supplier or a wrapped policy in a retry.
//
// Possible error(s): ErrDelayValidation, ErrTriesValidation, ErrCommandRequired,
// ErrUnhandledError, ErrMaxTriesExceeded.
func (p Policy) Run(metric core.Metric) error {
	if err := validate(p); err != nil {
		return err
	}

	m := Metric{ID: p.ServiceID, StartedAt: time.Now(), Executions: make([]struct {
		Iteration  int
		StartedAt  time.Time
		FinishedAt time.Time
		Duration   time.Duration
		Error      error
	}, 0)}
	done := false

	for i := 0; i < p.Tries; i++ {
		turn := i + 1
		m.Tries = turn
		exec := struct {
			Iteration  int
			StartedAt  time.Time
			FinishedAt time.Time
			Duration   time.Duration
			Error      error
		}{}

		exec.Iteration = turn

		if p.BeforeTry != nil {
			p.BeforeTry(p, turn)
		}

		exec.StartedAt = time.Now()
		err := execute(p, metric)
		err = pickError(err, metric)

		exec.Error = err
		exec.FinishedAt = time.Now()
		m.FinishedAt = time.Now()
		exec.Duration = exec.FinishedAt.Sub(exec.StartedAt)
		m.Executions = append(m.Executions, exec)

		if p.AfterTry != nil {
			p.AfterTry(p, turn, err)
		}

		if !handledError(p, err) || !handledError(p, metric.MetricError()) {
			m.Status = 1
			m.Error = ErrUnhandledError
			metric[reflect.TypeOf(m).String()] = m

			return ErrUnhandledError
		}

		if err == nil {
			done = true
			break
		}

		time.Sleep(p.Delay)
	}

	m.FinishedAt = time.Now()

	if !done {
		m.Status = 1
		m.Error = ErrMaxTriesExceeded
		metric[reflect.TypeOf(m).String()] = m

		return ErrMaxTriesExceeded
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

// ServiceID returns the service id registered to the policy binded to this metric.
func (m Metric) ServiceID() string {
	return m.ID
}

// PolicyDuration returns the policy execution duration.
// As this metric may have one or more executions, duration calculation is done by iterating
// through all execution metrics, summing the return of each call to PolicyDuration.
//
// Returns the amount of time spent for all policies to execute.
func (m Metric) PolicyDuration() time.Duration {
	sum := time.Duration(0)
	for _, exec := range m.Executions {
		sum += exec.Duration
	}

	return sum
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

func handledError(p Policy, err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func validate(p Policy) error {
	switch {
	case p.Delay < MinDelay:
		return ErrDelayValidation
	case p.Tries < MinTries:
		return ErrTriesValidation
	case p.Command == nil && p.Policy == nil:
		return ErrCommandRequired
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
