package timeout

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrTimeoutValidation = fmt.Errorf("timeout must be >= %d", MinTimeout)
	ErrExecutionTimedOut = errors.New("execution timed out")
	ErrCommandRequired   = errors.New("command nor wrapped policy provided")
)

type Policy struct {
	ServiceID     string
	Timeout       time.Duration
	BeforeTimeout func(p Policy)
	AfterTimeout  func(p Policy, err error)
	Command       core.Command
	Policy        core.PolicySupplier
}

type Metric struct {
	ID         string
	Status     int
	StartedAt  time.Time
	FinishedAt time.Time
	Error      error
}

const MinTimeout = 0

func New(serviceID string) Policy {
	return Policy{
		ServiceID: serviceID,
		Timeout:   0,
	}
}

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

func (m Metric) ServiceID() string {
	return m.ID
}

func (m Metric) PolicyDuration() time.Duration {
	return m.FinishedAt.Sub(m.StartedAt)
}

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
