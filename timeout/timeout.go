package timeout

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrTimeoutError           = fmt.Errorf("timeout must be >= %d", MinTimeout)
	ErrExecutionTimedOutError = errors.New("execution timed out")
	ErrCommandRequiredError   = errors.New("command is required")
)

type Policy struct {
	ServiceID     string
	Timeout       time.Duration
	BeforeTimeout func(p Policy)
	AfterTimeout  func(p Policy, err error)
	Command       core.Command
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
	if p.Command == nil {
		return ErrCommandRequiredError
	}

	return runPolicy(metric, p, func(core.Metric) error { return p.Command() })
}

func (p Policy) RunPolicy(metric core.Metric, supplier core.PolicySupplier) error {
	return runPolicy(metric, p, supplier.Run)
}

func runPolicy(metric core.Metric, parent Policy, yield func(core.Metric) error) error {
	if err := validate(parent); err != nil {
		return err
	}

	m := Metric{ID: parent.ServiceID, StartedAt: time.Now()}
	if parent.BeforeTimeout != nil {
		parent.BeforeTimeout(parent)
	}

	cerr := make(chan error)
	c := make(chan string)
	go executeCommand(cerr, c, metric, yield)

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
		case <-time.After(parent.Timeout):
			merror = ErrExecutionTimedOutError
			m.Error = ErrExecutionTimedOutError
			m.Status = 1

			break waiting
		}
	}

	if parent.AfterTimeout != nil {
		parent.AfterTimeout(parent, m.Error)
	}
	m.FinishedAt = time.Now()
	metric[reflect.TypeOf(m).String()] = m

	return merror
}

func executeCommand(cerr chan error, c chan string, metric core.Metric,
	yield func(core.Metric) error) {
	c <- "start"
	lerr := yield(metric)

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
	if p.Timeout < MinTimeout {
		return ErrTimeoutError
	}

	return nil
}
