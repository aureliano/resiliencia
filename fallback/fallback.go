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
	if parent.BeforeFallBack != nil {
		parent.BeforeFallBack(parent)
	}

	err := yield(metric)

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
	if p.FallBackHandler == nil {
		return ErrNoFallBackHandler
	}

	return nil
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
