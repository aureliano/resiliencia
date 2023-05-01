package timeout

import (
	"errors"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrTimeoutError = errors.New("timeout must be >= 0")
)

type Policy struct {
	ServiceID     string
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

func New(serviceID string) Policy {
	return Policy{
		ServiceID: serviceID,
		Timeout:   0,
	}
}

func (p Policy) Run(cmd core.Command) (*Metric, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	m := Metric{ID: p.ServiceID, StartedAt: time.Now()}
	if p.BeforeTimeout != nil {
		p.BeforeTimeout(p)
	}

	var err, cmdErr error
	c := make(chan string)
	go func() {
		c <- "start"
		cmdErr = cmd()
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
