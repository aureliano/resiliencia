package retry

import (
	"errors"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrExceededTries  = errors.New("max tries reached")
	ErrUnhandledError = errors.New("unhandled error")
	ErrDelayError     = errors.New("delay must be >= 0")
	ErrTriesError     = errors.New("tries must be > 0")
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
	Executions []struct {
		Iteration  int
		StartedAt  time.Time
		FinishedAt time.Time
		Duration   time.Duration
		Error      error
	}
}

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
	}, parent.Tries)}
	done := false

	for i := 0; i < parent.Tries; i++ {
		turn := i + 1
		m.Tries = turn
		m.Executions[i].Iteration = turn

		if parent.BeforeTry != nil {
			parent.BeforeTry(parent, turn)
		}

		m.Executions[i].StartedAt = time.Now()
		mr, err := yield()
		if mr != nil {
			metric[reflect.TypeOf(&mr).String()] = mr
		}

		m.Executions[i].Error = err
		m.Executions[i].FinishedAt = time.Now()
		m.FinishedAt = time.Now()
		m.Executions[i].Duration = m.Executions[i].FinishedAt.Sub(m.Executions[i].StartedAt)

		if parent.AfterTry != nil {
			parent.AfterTry(parent, turn, err)
		}

		if err != nil && !handledError(parent.Errors, err) {
			m.Status = 1
			metric[reflect.TypeOf(m).String()] = &m

			return ErrUnhandledError
		}

		if err == nil {
			done = true
			break
		}

		time.Sleep(parent.Delay)
	}

	m.FinishedAt = time.Now()
	metric[reflect.TypeOf(m).String()] = &m

	if !done {
		m.Status = 1
		return ErrExceededTries
	}

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
	return m.Status == 0
}

func handledError(errors []error, err error) bool {
	return core.ErrorInErrors(errors, err)
}

func validate(p Policy) error {
	switch {
	case p.Delay < 0:
		return ErrDelayError
	case p.Tries <= 0:
		return ErrTriesError
	default:
		return nil
	}
}
