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

func (p Policy) Run() (core.MetricRecorder, error) {
	if p.Command == nil {
		return nil, ErrCommandRequiredError
	}

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

	m := Metric{ID: parent.ServiceID, StartedAt: time.Now()}
	if parent.BeforeTimeout != nil {
		parent.BeforeTimeout(parent)
	}

	cmr := make(chan core.MetricRecorder)
	cerr := make(chan error)
	c := make(chan string)
	go executeCommand(cmr, cerr, c, yield)

	var mr core.MetricRecorder
	var merror error

waiting:
	for {
		select {
		case mr = <-cmr:
			if mr != nil {
				metric[reflect.TypeOf(mr).String()] = mr
			}
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

func executeCommand(cmr chan core.MetricRecorder, cerr chan error, c chan string,
	yield func() (core.MetricRecorder, error)) {
	c <- "start"
	lmr, lerr := yield()

	if lmr != nil {
		cmr <- lmr
	}

	if lerr != nil {
		cerr <- lerr
	}
	close(cmr)
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
