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

func TestChainerImplementsChainer(t *testing.T) {
	d := resiliencia.ChainOfResponsibility{}
	i := reflect.TypeOf((*resiliencia.Chainer)(nil)).Elem()

	assert.True(t, reflect.TypeOf(d).Implements(i))
}

func TestChainerExecutePolicyRequired(t *testing.T) {
	c := resiliencia.Chain()

	_, err := c.Execute(func() error { return nil })
	assert.ErrorIs(t, err, resiliencia.ErrPolicyRequired)

	c = resiliencia.Chain(nil, nil)

	_, err = c.Execute(func() error { return nil })
	assert.ErrorIs(t, err, resiliencia.ErrPolicyRequired)
}

func TestChainerExecuteSupplierRequired(t *testing.T) {
	c := resiliencia.Chain(fallback.New("srvname"), nil, retry.New("srvname"))

	_, err := c.Execute(nil)
	assert.ErrorIs(t, err, resiliencia.ErrSupplierRequired)
}

func TestChainerExecuteFallback(t *testing.T) {
	f := fallback.New("service-id")
	f.FallBackHandler = func(err error) {}
	c := resiliencia.Chain(f)

	metric, err := c.Execute(func() error { return nil })
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

func TestChainerExecuteFallbackWithCircuitBreaker(t *testing.T) {
	id := "service-id"
	f := fallback.New(id)
	f.FallBackHandler = func(err error) {}
	c := resiliencia.Chain(f, circuitbreaker.New(id))

	metric, err := c.Execute(func() error { return nil })
	assert.Nil(t, err)

	r := metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
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

func TestChainerExecuteFallbackWithCircuitBreakerAndRetry(t *testing.T) {
	id := "service-id"
	f := fallback.New(id)
	f.FallBackHandler = func(err error) {}
	c := resiliencia.Chain(f, circuitbreaker.New(id), retry.New(id))

	metric, err := c.Execute(func() error { return nil })
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

func TestChainerExecuteFallbackWithCircuitBreakerAndRetryAndTimeout(t *testing.T) {
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

	c := resiliencia.Chain(tmp, f, rt, cb, f, tmp)

	metric, err := c.Execute(func() error { return nil })
	assert.Nil(t, err)
	assert.Equal(t, "tmfbcbrtfbtm", sb.String())

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

func TestChainerExecuteCircuitBreaker(t *testing.T) {
	c := resiliencia.Chain(circuitbreaker.New("service-id"))

	metric, err := c.Execute(func() error { return nil })
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

func TestChainerExecuteCircuitBreakerWithRetry(t *testing.T) {
	id := "service-id"
	c := resiliencia.Chain(circuitbreaker.New(id), retry.New(id))

	metric, err := c.Execute(func() error { return nil })
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

func TestChainerExecuteCircuitBreakerWithRetryAndTimeout(t *testing.T) {
	id := "service-id"
	tmp := timeout.New(id)
	tmp.Timeout = time.Second * 5
	c := resiliencia.Chain(circuitbreaker.New(id), retry.New(id), tmp)

	metric, err := c.Execute(func() error { return nil })
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

func TestChainerExecuteRetry(t *testing.T) {
	id := "service-id"
	c := resiliencia.Chain(retry.New(id))

	metric, err := c.Execute(func() error { return nil })
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

func TestChainerExecuteRetryWithTimeout(t *testing.T) {
	id := "service-id"
	tmp := timeout.New(id)
	tmp.Timeout = time.Second * 5
	c := resiliencia.Chain(retry.New(id), tmp)

	metric, err := c.Execute(func() error { return nil })
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

func TestChainerExecuteTimeout(t *testing.T) {
	id := "service-id"
	tmp := timeout.New(id)
	tmp.Timeout = time.Second * 5
	c := resiliencia.Chain(tmp)

	metric, err := c.Execute(func() error { return nil })
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
