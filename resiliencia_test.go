package resiliencia_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aureliano/resiliencia"
	"github.com/aureliano/resiliencia/circuitbreaker"
	"github.com/aureliano/resiliencia/fallback"
	"github.com/aureliano/resiliencia/retry"
	"github.com/aureliano/resiliencia/timeout"
	"github.com/stretchr/testify/assert"
)

func TestDecorate(t *testing.T) {
	d := resiliencia.Decorate(func() error { return nil })
	assert.NotNil(t, d)
	s, _ := d.(resiliencia.Decoration)
	assert.NotNil(t, s.Supplier)

	d = d.WithRetry(retry.Policy{ServiceID: "serviceId", Tries: 3, Delay: time.Second * 15})
	s, _ = d.(resiliencia.Decoration)

	assert.Equal(t, "serviceId", s.Retry.ServiceID)
	assert.Equal(t, 3, s.Retry.Tries)
	assert.Equal(t, time.Second*15, s.Retry.Delay)

	d = d.WithTimeout(timeout.Policy{ServiceID: "serviceId", Timeout: time.Minute * 2})
	s, _ = d.(resiliencia.Decoration)

	assert.Equal(t, "serviceId", s.Timeout.ServiceID)
	assert.Equal(t, time.Minute*2, s.Timeout.Timeout)

	d = d.WithFallback([]fallback.Policy{{ServiceID: "serviceId", FallBackHandler: func(err error) {}}})
	s, _ = d.(resiliencia.Decoration)

	assert.Equal(t, "serviceId", s.Fallback[0].ServiceID)
	assert.NotNil(t, s.Fallback[0].FallBackHandler)

	d = d.WithCircuitBreaker(circuitbreaker.Policy{
		ServiceID:       "serviceId",
		ThresholdErrors: 5,
		ResetTimeout:    time.Second * 45,
		Errors:          []error{fmt.Errorf("")},
	})
	s, _ = d.(resiliencia.Decoration)

	assert.Equal(t, "serviceId", s.CircuitBreaker.ServiceID)
	assert.Equal(t, 5, s.CircuitBreaker.ThresholdErrors)
	assert.Equal(t, time.Second*45, s.CircuitBreaker.ResetTimeout)
	assert.Len(t, s.CircuitBreaker.Errors, 1)
}
