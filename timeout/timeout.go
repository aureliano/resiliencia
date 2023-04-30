package timeout

import (
	"context"
	"errors"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrTimeoutError = errors.New("timeout must be >= 0")
)

type Policy struct {
	Timeout       time.Duration
	BeforeTimeout func(p Policy)
	AfterTimeout  func(p Policy, err error)
}

type Metric struct {
	ID         string
	Status     int
	StartedAt  time.Time
	FinishedAt time.Time
	Error      error
}

func New() Policy {
	return Policy{
		Timeout: 0,
	}
}

func (p Policy) Run(ctx context.Context, cmd core.Command) (*Metric, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	m := Metric{StartedAt: time.Now()}
	if p.BeforeTimeout != nil {
		p.BeforeTimeout(p)
	}

	var err, cmdErr error
	c := make(chan string)
	go func() {
		c <- "start"
		cmdErr = cmd(ctx)
		m.Error = cmdErr
		c <- "done"
	}()

waiting:
	for {
		select {
		case str := <-c:
			if str == "done" {
				break waiting
			}
		case <-time.After(p.Timeout):
			err = ErrTimeoutError
			m.Status = 1
			break waiting
		}
	}

	if p.AfterTimeout != nil {
		p.AfterTimeout(p, cmdErr)
	}
	m.FinishedAt = time.Now()

	return &m, err
}

func (p Policy) validate() error {
	if p.Timeout < 0 {
		return ErrTimeoutError
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
