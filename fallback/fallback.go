package fallback

import (
	"errors"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrNoFallBackHandler    = errors.New("no fallback handler")
	ErrCommandRequiredError = errors.New("command is required")
	ErrUnhandledError       = errors.New("unhandled error")
)

type Policy struct {
	ServiceID       string
	Errors          []error
	FallBackHandler func(err error)
	BeforeFallBack  func(p Policy)
	AfterFallBack   func(p Policy, err error)
	Command         core.Command
}

type Metric struct {
	ID         string
	Status     int
	StartedAt  time.Time
	FinishedAt time.Time
	Error      error
}

func New(serviceID string) Policy {
	return Policy{ServiceID: serviceID}
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
	if parent.BeforeFallBack != nil {
		parent.BeforeFallBack(parent)
	}

	mr, err := yield()
	if mr != nil {
		metric[reflect.TypeOf(mr).String()] = mr
	}

	if parent.AfterFallBack != nil {
		parent.AfterFallBack(parent, err)
	}
	m.FinishedAt = time.Now()

	if err != nil && !handledError(parent, err) {
		m.Status = 1
		m.Error = ErrUnhandledError
		metric[reflect.TypeOf(m).String()] = m

		return ErrUnhandledError
	}

	if err != nil {
		parent.FallBackHandler(err)
	}
	metric[reflect.TypeOf(m).String()] = m

	return nil
}

func handledError(p Policy, err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func validate(p Policy) error {
	switch {
	case p.FallBackHandler == nil:
		return ErrNoFallBackHandler
	case p.Command == nil:
		return ErrCommandRequiredError
	default:
		return nil
	}
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
