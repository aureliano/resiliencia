package fallback

import (
	"errors"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrNoFallBackHandler = errors.New("no fallback handler")
	ErrUnhandledError    = errors.New("unhandled error")
)

type Policy struct {
	Errors          []error
	FallBackHandler func(err error)
	BeforeFallBack  func(p Policy)
	AfterFallBack   func(p Policy, err error)
}

type Metric struct {
	ID         string
	Status     int
	StartedAt  time.Time
	FinishedAt time.Time
	Error      error
}

func New() Policy {
	return Policy{}
}

func (p Policy) Run(cmd core.Command) (*Metric, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	m := Metric{StartedAt: time.Now()}
	if p.BeforeFallBack != nil {
		p.BeforeFallBack(p)
	}

	err := cmd()
	m.Error = err

	if p.AfterFallBack != nil {
		p.AfterFallBack(p, err)
	}
	m.FinishedAt = time.Now()

	if err != nil && !p.handledError(err) {
		m.Status = 1
		return &m, ErrUnhandledError
	}

	if err != nil {
		p.FallBackHandler(err)
	}

	return &m, nil
}

func (p Policy) handledError(err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func (p Policy) validate() error {
	if p.FallBackHandler == nil {
		return ErrNoFallBackHandler
	}

	return nil
}

func (m *Metric) ServiceID() string {
	return m.ID
}

func (m *Metric) PolicyDuration() time.Duration {
	return m.FinishedAt.Sub(m.StartedAt)
}

func (m *Metric) Success() bool {
	return m.Status == 0
}
