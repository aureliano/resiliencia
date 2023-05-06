package retry

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrDelayError           = fmt.Errorf("delay must be >= %d", MinDelay)
	ErrTriesError           = fmt.Errorf("tries must be >= %d", MinTries)
	ErrMaxTriesExceeded     = errors.New("max tries reached")
	ErrUnhandledError       = errors.New("unhandled error")
	ErrCommandRequiredError = errors.New("command is required")
)

type Policy struct {
	ServiceID string
	Tries     int
	Delay     time.Duration
	Errors    []error
	BeforeTry func(p Policy, try int)
	AfterTry  func(p Policy, try int, err error)
	Command   core.Command
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

func (p Policy) Run() (core.MetricRecorder, error) {
	metric := core.NewMetric()
	err := runPolicy(metric, p, func() (core.MetricRecorder, error) { return nil, p.Command() })
	m := metric[reflect.TypeOf(Metric{}).String()]

	return m, err
}

func (p Policy) RunPolicy(metric core.Metric, supplier core.PolicySupplier) error {
	return runPolicy(metric, p, supplier.Run)
}

func runPolicy(metric core.Metric, parent Policy, yield func() (core.MetricRecorder, error)) error {
	if err := validate(parent); err != nil {
		return err
	}

	m := Metric{ID: parent.ServiceID, StartedAt: time.Now(), Executions: make([]struct {
		Iteration  int
		StartedAt  time.Time
		FinishedAt time.Time
		Duration   time.Duration
		Error      error
	}, 0)}
	done := false

	for i := 0; i < parent.Tries; i++ {
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

		if parent.BeforeTry != nil {
			parent.BeforeTry(parent, turn)
		}

		exec.StartedAt = time.Now()
		mr, err := yield()
		if mr != nil {
			metric[reflect.TypeOf(mr).String()] = mr
		}

		exec.Error = err
		exec.FinishedAt = time.Now()
		m.FinishedAt = time.Now()
		exec.Duration = exec.FinishedAt.Sub(exec.StartedAt)
		m.Executions = append(m.Executions, exec)

		if parent.AfterTry != nil {
			parent.AfterTry(parent, turn, err)
		}

		if err != nil && !handledError(parent, err) {
			m.Status = 1
			m.Error = ErrUnhandledError
			metric[reflect.TypeOf(m).String()] = m

			return ErrUnhandledError
		}

		if err == nil {
			done = true
			break
		}

		time.Sleep(parent.Delay)
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
		return ErrDelayError
	case p.Tries < MinTries:
		return ErrTriesError
	case p.Command == nil:
		return ErrCommandRequiredError
	default:
		return nil
	}
}
