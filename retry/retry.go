package retry

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrDelayValidation  = fmt.Errorf("delay must be >= %d", MinDelay)
	ErrTriesValidation  = fmt.Errorf("tries must be >= %d", MinTries)
	ErrMaxTriesExceeded = errors.New("max tries reached")
	ErrUnhandledError   = errors.New("unhandled error")
	ErrCommandRequired  = errors.New("command nor wrapped policy provided")
)

type Policy struct {
	ServiceID string
	Tries     int
	Delay     time.Duration
	Errors    []error
	BeforeTry func(p Policy, try int)
	AfterTry  func(p Policy, try int, err error)
	Command   core.Command
	Policy    core.PolicySupplier
}

type Metric struct {
	ID         string
	Tries      int
	Status     int
	StartedAt  time.Time
	FinishedAt time.Time
	Error      error
	Executions []struct {
		Iteration  int
		StartedAt  time.Time
		FinishedAt time.Time
		Duration   time.Duration
		Error      error
	}
}

const (
	MinDelay = 0
	MinTries = 1
)

func New(serviceID string) Policy {
	return Policy{
		ServiceID: serviceID,
		Tries:     1,
		Delay:     0,
	}
}

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

		exec.Error = err
		exec.FinishedAt = time.Now()
		m.FinishedAt = time.Now()
		exec.Duration = exec.FinishedAt.Sub(exec.StartedAt)
		m.Executions = append(m.Executions, exec)

		if p.AfterTry != nil {
			p.AfterTry(p, turn, err)
		}

		if err != nil && !handledError(p, err) {
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

func execute(p Policy, metric core.Metric) error {
	if p.Command != nil && p.Policy == nil {
		return p.Command()
	}

	return p.Policy.Run(metric)
}

func (m Metric) ServiceID() string {
	return m.ID
}

func (m Metric) PolicyDuration() time.Duration {
	sum := time.Duration(0)
	for _, exec := range m.Executions {
		sum += exec.Duration
	}

	return sum
}

func (m Metric) Success() bool {
	return (m.Status == 0) && (m.Error == nil)
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
