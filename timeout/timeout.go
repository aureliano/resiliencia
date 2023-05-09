package timeout

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	// Policy timeout is less than minimum required.
	ErrTimeoutValidation = fmt.Errorf("timeout must be >= %d", MinTimeout)

	// The execution exceeded maximun defined time.
	ErrExecutionTimedOut = errors.New("execution timed out")

	// No command nor wrapped policy is set.
	ErrCommandRequired = errors.New("command nor wrapped policy provided")
)

// Policy defines the timeout algorithm execution policy.
type Policy struct {
	// The registered service id.
	ServiceID string

	// Time to wait until the run times out.
	Timeout time.Duration

	// Function called before execution.
	BeforeTimeout func(p Policy)

	// Function called after execution.
	AfterTimeout func(p Policy, err error)

	// The command supplier.
	Command core.Command

	// Any policy that will be wrapped by this one.
	Policy core.PolicySupplier
}

// Metric keeps the running state of the timeout.
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

// Minimum expected to be set on Timeout field of a timeout policy.
const MinTimeout = 0

// New creates a timeout policy with default values set.
func New(serviceID string) Policy {
	return Policy{
		ServiceID: serviceID,
		Timeout:   0,
	}
}

// Run executes a command supplier or a wrapped policy in a timeout.
//
// Possible error(s): ErrTimeoutValidation, ErrCommandRequired, ErrExecutionTimedOut.
func (p Policy) Run(metric core.Metric) error {
	if err := validate(p); err != nil {
		return err
	}

	m := Metric{ID: p.ServiceID, StartedAt: time.Now()}
	if p.BeforeTimeout != nil {
		p.BeforeTimeout(p)
	}

	cerr := make(chan error)
	c := make(chan string)
	go executeCommand(cerr, c, metric, p)

	var merror error

waiting:
	for {
		select {
		case e := <-cerr:
			if e != nil {
				m.Error = e
				m.Status = 1
			}
		case str := <-c:
			if str == "done" {
				break waiting
			}
		case <-time.After(p.Timeout):
			merror = ErrExecutionTimedOut
			m.Error = ErrExecutionTimedOut
			m.Status = 1

			break waiting
		}
	}

	if p.AfterTimeout != nil {
		p.AfterTimeout(p, m.Error)
	}
	m.FinishedAt = time.Now()
	metric[reflect.TypeOf(m).String()] = m

	return merror
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

func executeCommand(cerr chan error, c chan string, metric core.Metric, p Policy) {
	c <- "start"

	var lerr error
	if p.Command != nil && p.Policy == nil {
		lerr = p.Command()
	} else {
		lerr = p.Policy.Run(metric)
	}

	if lerr != nil {
		cerr <- lerr
	}
	close(cerr)

	c <- "done"
	close(c)
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

func validate(p Policy) error {
	switch {
	case p.Timeout < MinTimeout:
		return ErrTimeoutValidation
	case p.Command == nil && p.Policy == nil:
		return ErrCommandRequired
	default:
		return nil
	}
}
