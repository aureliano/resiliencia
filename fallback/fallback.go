package fallback

import (
	"errors"
	"reflect"
	"time"

	"github.com/aureliano/resiliencia/core"
)

var (
	ErrNoFallBackHandler    = errors.New("no fallback handler")
	ErrCommandRequiredError = errors.New("command nor wrapped policy provided")
	ErrUnhandledError       = errors.New("unhandled error")
)

type Policy struct {
	ServiceID       string
	Errors          []error
	FallBackHandler func(err error)
	BeforeFallBack  func(p Policy)
	AfterFallBack   func(p Policy, err error)
	Command         core.Command
	Policy          core.PolicySupplier
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
	if err := validate(p); err != nil {
		return err
	}

	m := Metric{ID: p.ServiceID, StartedAt: time.Now()}
	if p.BeforeFallBack != nil {
		p.BeforeFallBack(p)
	}

	err := execute(p, metric)

	if p.AfterFallBack != nil {
		p.AfterFallBack(p, err)
	}
	m.FinishedAt = time.Now()

	if err != nil && !handledError(p, err) {
		m.Status = 1
		m.Error = ErrUnhandledError
		metric[reflect.TypeOf(m).String()] = m

		return ErrUnhandledError
	}

	if err != nil {
		p.FallBackHandler(err)
	}
	metric[reflect.TypeOf(m).String()] = m

	return nil
}

func (p Policy) WithCommand(command core.Command) core.PolicySupplier {
	p.Command = command
	return p
}

func (p Policy) WithPolicy(policy core.PolicySupplier) core.PolicySupplier {
	p.Policy = policy
	return p
}

func execute(p Policy, metric core.Metric) error {
	if p.Command != nil && p.Policy == nil {
		return p.Command()
	}

	return p.Policy.Run(metric)
}

func handledError(p Policy, err error) bool {
	return core.ErrorInErrors(p.Errors, err)
}

func validate(p Policy) error {
	switch {
	case p.FallBackHandler == nil:
		return ErrNoFallBackHandler
	case p.Command == nil && p.Policy == nil:
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
