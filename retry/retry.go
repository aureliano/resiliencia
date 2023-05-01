package retry

import (
	"errors"
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
	Tries     int
	Delay     time.Duration
	Errors    []error
	BeforeTry func(p Policy, try int)
	AfterTry  func(p Policy, try int, err error)
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

func New() Policy {
	return Policy{
		Tries: 1,
		Delay: 0,
	}
}

func (p Policy) Run(cmd core.Command) (*Metric, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	m := Metric{StartedAt: time.Now(), Executions: make([]struct {
		Iteration  int
		StartedAt  time.Time
		FinishedAt time.Time
		Duration   time.Duration
		Error      error
	}, p.Tries)}
	done := false

	for i := 0; i < p.Tries; i++ {
		turn := i + 1
		m.Tries = turn
		m.Executions[i].Iteration = turn

		if p.BeforeTry != nil {
			p.BeforeTry(p, turn)
		}

		m.Executions[i].StartedAt = time.Now()
		err := cmd()
		m.Executions[i].Error = err
		m.Executions[i].FinishedAt = time.Now()
		m.FinishedAt = time.Now()
		m.Executions[i].Duration = m.Executions[i].FinishedAt.Sub(m.Executions[i].StartedAt)

		if p.AfterTry != nil {
			p.AfterTry(p, turn, err)
		}

		if err != nil && !p.handledError(err) {
			m.Status = 1
			return &m, ErrUnhandledError
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
		return &m, ErrExceededTries
	}

	return &m, nil
}

func (p Policy) handledError(err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func (p Policy) validate() error {
	switch {
	case p.Delay < 0:
		return ErrDelayError
	case p.Tries <= 0:
		return ErrTriesError
	default:
		return nil
	}
}

func (m *Metric) ServiceID() string {
	return m.ID
}

func (m *Metric) PolicyDuration() time.Duration {
	sum := time.Duration(0)
	for _, exec := range m.Executions {
		sum += exec.Duration
	}

	return sum
}

func (m *Metric) Success() bool {
	return m.Status == 0
}
