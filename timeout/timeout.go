package timeout

import (
	"errors"
	"reflect"
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
	Command       core.Command
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

	m := Metric{ID: parent.ServiceID, StartedAt: time.Now()}
	if parent.BeforeTimeout != nil {
		parent.BeforeTimeout(parent)
	}

	var err, cmdErr error
	c := make(chan string)
	go func() {
		c <- "start"
		var mr core.MetricRecorder

		mr, cmdErr = yield()
		if mr != nil {
			metric[reflect.TypeOf(&mr).String()] = mr
		}

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
		case <-time.After(parent.Timeout):
			err = ErrTimeoutError
			m.Status = 1
			break waiting
		}
	}

	if parent.AfterTimeout != nil {
		parent.AfterTimeout(parent, cmdErr)
	}
	m.FinishedAt = time.Now()
	metric[reflect.TypeOf(m).String()] = &m

	return err
}

func (m Metric) ServiceID() string {
	return m.ID
}

func (m Metric) PolicyDuration() time.Duration {
	return m.FinishedAt.Sub(m.StartedAt)
}

func (m Metric) Success() bool {
	return m.Status == 0
}

func validate(p Policy) error {
	if p.Timeout < 0 {
		return ErrTimeoutError
	}

	return nil
}
