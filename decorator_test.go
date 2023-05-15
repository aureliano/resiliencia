package resiliencia_test

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/aureliano/resiliencia"
	"github.com/aureliano/resiliencia/circuitbreaker"
	"github.com/aureliano/resiliencia/fallback"
	"github.com/aureliano/resiliencia/retry"
	"github.com/aureliano/resiliencia/timeout"
	"github.com/stretchr/testify/assert"
)

func TestDecorationImplementsDecorator(t *testing.T) {
	d := resiliencia.Decoration{}
	i := reflect.TypeOf((*resiliencia.Decorator)(nil)).Elem()

	assert.True(t, reflect.TypeOf(d).Implements(i))
}

func TestDecoratorExecutePolicyRequired(t *testing.T) {
	d := resiliencia.Decorate(func() error { return nil })

	_, err := d.Execute()
	assert.ErrorIs(t, err, resiliencia.ErrPolicyRequired)
}

func TestDecoratorExecuteSupplierRequired(t *testing.T) {
	d := resiliencia.Decorate(nil)
	d = d.WithRetry(retry.New("id"))

	_, err := d.Execute()
	assert.ErrorIs(t, err, resiliencia.ErrSupplierRequired)
}

func TestDecoratorExecuteAnyWrappedPolicyWithCommand(t *testing.T) {
	d := resiliencia.Decorate(func() error { return nil })
	d = d.WithRetry(retry.New("id"))
	tm := timeout.New("id")
	tm.Command = func() error { return nil }
	d = d.WithTimeout(tm)

	_, err := d.Execute()
	assert.ErrorIs(t, err, resiliencia.ErrWrappedPolicyWithCommand)
}

func TestDecoratorExecuteAnyWrappedPolicyWithNestedPolicy(t *testing.T) {
	d := resiliencia.Decorate(func() error { return nil })
	d = d.WithRetry(retry.New("id"))
	tm := timeout.New("id")
	tm.Policy = fallback.New("id")
	d = d.WithTimeout(tm)

	_, err := d.Execute()
	assert.ErrorIs(t, err, resiliencia.ErrWrappedPolicyWithNestedPolicy)
}

func TestDecoratorExecuteFallback(t *testing.T) {
	d := resiliencia.Decorate(func() error { return nil })
	f := fallback.New("service-id")
	f.FallBackHandler = func(err error) {}
	d = d.WithFallback(f)

	metric, err := d.Execute()
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(fallback.Metric{}).String()]
	m, _ := r.(fallback.Metric)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, time.Now())
	assert.Less(t, m.FinishedAt, time.Now())
	assert.Nil(t, m.Error)
	assert.Greater(t, m.PolicyDuration(), time.Duration(0))
}

func TestDecoratorExecuteFallbackWithCircuitBreaker(t *testing.T) {
	id := "service-id"
	d := resiliencia.Decorate(func() error { return nil })
	f := fallback.New(id)
	f.FallBackHandler = func(err error) {}
	d = d.WithFallback(f)
	d = d.WithCircuitBreaker(circuitbreaker.New(id))

	metric, err := d.Execute()
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	assert.NotNil(t, r)
	cbm, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "service-id", cbm.ID)
	assert.Equal(t, 0, cbm.Status)
	assert.Less(t, cbm.StartedAt, time.Now())
	assert.Less(t, cbm.FinishedAt, time.Now())
	assert.Nil(t, cbm.Error)
	assert.Equal(t, circuitbreaker.ClosedState, cbm.State)
	assert.Equal(t, 0, cbm.ErrorCount)
	assert.Greater(t, cbm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(fallback.Metric{}).String()]
	m, _ := r.(fallback.Metric)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, time.Now())
	assert.Less(t, m.FinishedAt, time.Now())
	assert.Nil(t, m.Error)
	assert.Greater(t, m.PolicyDuration(), time.Duration(0))
}

func TestDecoratorExecuteFallbackWithCircuitBreakerAndRetry(t *testing.T) {
	id := "service-id"
	d := resiliencia.Decorate(func() error { return nil })
	f := fallback.New(id)
	f.FallBackHandler = func(err error) {}
	d = d.WithFallback(f)
	d = d.WithCircuitBreaker(circuitbreaker.New(id))
	d = d.WithRetry(retry.New(id))

	metric, err := d.Execute()
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(retry.Metric{}).String()]
	rm, _ := r.(retry.Metric)

	assert.Equal(t, "service-id", rm.ID)
	assert.Equal(t, 0, rm.Status)
	assert.Less(t, rm.StartedAt, time.Now())
	assert.Less(t, rm.FinishedAt, time.Now())
	assert.Nil(t, rm.Error)
	assert.Equal(t, 1, rm.Tries)
	assert.Greater(t, rm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbm, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "service-id", cbm.ID)
	assert.Equal(t, 0, cbm.Status)
	assert.Less(t, cbm.StartedAt, time.Now())
	assert.Less(t, cbm.FinishedAt, time.Now())
	assert.Nil(t, cbm.Error)
	assert.Equal(t, circuitbreaker.ClosedState, cbm.State)
	assert.Equal(t, 0, cbm.ErrorCount)
	assert.Greater(t, cbm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(fallback.Metric{}).String()]
	m, _ := r.(fallback.Metric)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, time.Now())
	assert.Less(t, m.FinishedAt, time.Now())
	assert.Nil(t, m.Error)
	assert.Greater(t, m.PolicyDuration(), time.Duration(0))
}

func TestDecoratorExecuteFallbackWithCircuitBreakerAndRetryAndTimeout(t *testing.T) {
	id := "service-id"
	var sb bytes.Buffer

	f := fallback.New(id)
	f.FallBackHandler = func(err error) {}
	f.AfterFallBack = func(p fallback.Policy, err error) { sb.WriteString("fb") }
	cb := circuitbreaker.New(id)
	cb.AfterCircuitBreaker = func(p circuitbreaker.Policy,
		status *circuitbreaker.CircuitBreaker, err error) {
		sb.WriteString("cb")
	}
	rt := retry.New(id)
	rt.AfterTry = func(p retry.Policy, try int, err error) { sb.WriteString("rt") }
	tmp := timeout.New(id)
	tmp.Timeout = time.Second * 5
	tmp.AfterTimeout = func(p timeout.Policy, err error) { sb.WriteString("tm") }

	metric, err := resiliencia.
		Decorate(func() error {
			return nil
		}).
		WithCircuitBreaker(cb).
		WithFallback(f).
		WithRetry(rt).
		WithTimeout(tmp).
		Execute()

	assert.Nil(t, err)
	assert.Equal(t, "tmrtcbfb", sb.String())

	r := metric[reflect.TypeOf(timeout.Metric{}).String()]
	tm, _ := r.(timeout.Metric)

	assert.Equal(t, "service-id", tm.ID)
	assert.Equal(t, 0, tm.Status)
	assert.Less(t, tm.StartedAt, time.Now())
	assert.Less(t, tm.FinishedAt, time.Now())
	assert.Nil(t, tm.Error)
	assert.Greater(t, tm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(retry.Metric{}).String()]
	rm, _ := r.(retry.Metric)

	assert.Equal(t, "service-id", rm.ID)
	assert.Equal(t, 0, rm.Status)
	assert.Less(t, rm.StartedAt, time.Now())
	assert.Less(t, rm.FinishedAt, time.Now())
	assert.Nil(t, rm.Error)
	assert.Equal(t, 1, rm.Tries)
	assert.Greater(t, rm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbm, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "service-id", cbm.ID)
	assert.Equal(t, 0, cbm.Status)
	assert.Less(t, cbm.StartedAt, time.Now())
	assert.Less(t, cbm.FinishedAt, time.Now())
	assert.Nil(t, cbm.Error)
	assert.Equal(t, circuitbreaker.ClosedState, cbm.State)
	assert.Equal(t, 0, cbm.ErrorCount)
	assert.Greater(t, cbm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(fallback.Metric{}).String()]
	m, _ := r.(fallback.Metric)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, time.Now())
	assert.Less(t, m.FinishedAt, time.Now())
	assert.Nil(t, m.Error)
	assert.Greater(t, m.PolicyDuration(), time.Duration(0))
}

func TestDecoratorExecuteCircuitBreaker(t *testing.T) {
	d := resiliencia.Decorate(func() error { return nil })
	d = d.WithCircuitBreaker(circuitbreaker.New("service-id"))

	metric, err := d.Execute()
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	m, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "service-id", m.ID)
	assert.Equal(t, 0, m.Status)
	assert.Less(t, m.StartedAt, time.Now())
	assert.Less(t, m.FinishedAt, time.Now())
	assert.Nil(t, m.Error)
	assert.Equal(t, circuitbreaker.ClosedState, m.State)
	assert.Equal(t, 0, m.ErrorCount)
	assert.Greater(t, m.PolicyDuration(), time.Duration(0))
}

func TestDecoratorExecuteCircuitBreakerWithRetry(t *testing.T) {
	id := "service-id"
	d := resiliencia.Decorate(func() error { return nil })
	d = d.WithCircuitBreaker(circuitbreaker.New(id))
	d = d.WithRetry(retry.New(id))

	metric, err := d.Execute()
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(retry.Metric{}).String()]
	rm, _ := r.(retry.Metric)

	assert.Equal(t, "service-id", rm.ID)
	assert.Equal(t, 0, rm.Status)
	assert.Less(t, rm.StartedAt, time.Now())
	assert.Less(t, rm.FinishedAt, time.Now())
	assert.Nil(t, rm.Error)
	assert.Equal(t, 1, rm.Tries)
	assert.Greater(t, rm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbm, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "service-id", cbm.ID)
	assert.Equal(t, 0, cbm.Status)
	assert.Less(t, cbm.StartedAt, time.Now())
	assert.Less(t, cbm.FinishedAt, time.Now())
	assert.Nil(t, cbm.Error)
	assert.Equal(t, circuitbreaker.ClosedState, cbm.State)
	assert.Equal(t, 0, cbm.ErrorCount)
	assert.Greater(t, cbm.PolicyDuration(), time.Duration(0))
}

func TestDecoratorExecuteCircuitBreakerWithRetryAndTimeout(t *testing.T) {
	id := "service-id"
	d := resiliencia.Decorate(func() error { return nil })
	d = d.WithCircuitBreaker(circuitbreaker.New(id))
	d = d.WithRetry(retry.New(id))
	tmp := timeout.New(id)
	tmp.Timeout = time.Second * 5
	d = d.WithTimeout(tmp)

	metric, err := d.Execute()
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(timeout.Metric{}).String()]
	tm, _ := r.(timeout.Metric)

	assert.Equal(t, "service-id", tm.ID)
	assert.Equal(t, 0, tm.Status)
	assert.Less(t, tm.StartedAt, time.Now())
	assert.Less(t, tm.FinishedAt, time.Now())
	assert.Nil(t, tm.Error)
	assert.Greater(t, tm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(retry.Metric{}).String()]
	rm, _ := r.(retry.Metric)

	assert.Equal(t, "service-id", rm.ID)
	assert.Equal(t, 0, tm.Status)
	assert.Less(t, rm.StartedAt, time.Now())
	assert.Less(t, rm.FinishedAt, time.Now())
	assert.Nil(t, rm.Error)
	assert.Equal(t, 1, rm.Tries)
	assert.Greater(t, rm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbm, _ := r.(circuitbreaker.Metric)

	assert.Equal(t, "service-id", cbm.ID)
	assert.Equal(t, 0, cbm.Status)
	assert.Less(t, cbm.StartedAt, time.Now())
	assert.Less(t, cbm.FinishedAt, time.Now())
	assert.Nil(t, cbm.Error)
	assert.Equal(t, circuitbreaker.ClosedState, cbm.State)
	assert.Equal(t, 0, cbm.ErrorCount)
	assert.Greater(t, cbm.PolicyDuration(), time.Duration(0))
}

func TestDecoratorExecuteRetry(t *testing.T) {
	id := "service-id"
	d := resiliencia.Decorate(func() error { return nil })
	d = d.WithRetry(retry.New(id))

	metric, err := d.Execute()
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(retry.Metric{}).String()]
	rm, _ := r.(retry.Metric)

	assert.Equal(t, "service-id", rm.ID)
	assert.Equal(t, 0, rm.Status)
	assert.Less(t, rm.StartedAt, time.Now())
	assert.Less(t, rm.FinishedAt, time.Now())
	assert.Nil(t, rm.Error)
	assert.Equal(t, 1, rm.Tries)
	assert.Greater(t, rm.PolicyDuration(), time.Duration(0))
}

func TestDecoratorExecuteRetryWithTimeout(t *testing.T) {
	id := "service-id"
	d := resiliencia.Decorate(func() error { return nil })
	d = d.WithRetry(retry.New(id))
	tmp := timeout.New(id)
	tmp.Timeout = time.Second * 5
	d = d.WithTimeout(tmp)

	metric, err := d.Execute()
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(timeout.Metric{}).String()]
	tm, _ := r.(timeout.Metric)

	assert.Equal(t, "service-id", tm.ID)
	assert.Equal(t, 0, tm.Status)
	assert.Less(t, tm.StartedAt, time.Now())
	assert.Less(t, tm.FinishedAt, time.Now())
	assert.Nil(t, tm.Error)
	assert.Greater(t, tm.PolicyDuration(), time.Duration(0))

	r = metric[reflect.TypeOf(retry.Metric{}).String()]
	rm, _ := r.(retry.Metric)

	assert.Equal(t, "service-id", rm.ID)
	assert.Equal(t, 0, rm.Status)
	assert.Less(t, rm.StartedAt, time.Now())
	assert.Less(t, rm.FinishedAt, time.Now())
	assert.Nil(t, rm.Error)
	assert.Equal(t, 1, rm.Tries)
	assert.Greater(t, rm.PolicyDuration(), time.Duration(0))
}

func TestDecoratorExecuteTimeout(t *testing.T) {
	id := "service-id"
	d := resiliencia.Decorate(func() error { return nil })
	tmp := timeout.New(id)
	tmp.Timeout = time.Second * 5
	d = d.WithTimeout(tmp)

	metric, err := d.Execute()
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(timeout.Metric{}).String()]
	tm, _ := r.(timeout.Metric)

	assert.Equal(t, "service-id", tm.ID)
	assert.Equal(t, 0, tm.Status)
	assert.Less(t, tm.StartedAt, time.Now())
	assert.Less(t, tm.FinishedAt, time.Now())
	assert.Nil(t, tm.Error)
	assert.Greater(t, tm.PolicyDuration(), time.Duration(0))
}
