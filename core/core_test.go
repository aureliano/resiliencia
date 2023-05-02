package core_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/stretchr/testify/assert"
)

type dummyMetric struct {
	status int
}

func (dummyMetric) ServiceID() string {
	return "anything"
}

func (dummyMetric) PolicyDuration() time.Duration {
	return time.Duration(time.Second * 55)
}

func (m dummyMetric) Success() bool {
	return m.status == 0
}

func TestMetricImplementsMetricRecorder(t *testing.T) {
	m := core.Metric{}
	i := reflect.TypeOf((*core.MetricRecorder)(nil)).Elem()

	assert.True(t, reflect.TypeOf(m).Implements(i))
}

func TestNewMetric(t *testing.T) {
	m := core.NewMetric()
	assert.NotNil(t, m)
	assert.Len(t, m, 0)
}

func TestServiceID(t *testing.T) {
	m := core.NewMetric()
	assert.Equal(t, "policy-chain", m.ServiceID())
}

func TestPolicyDuration(t *testing.T) {
	m := core.NewMetric()
	m["test"] = dummyMetric{}

	assert.EqualValues(t, time.Second*55, m.PolicyDuration())
}

func TestSuccess(t *testing.T) {
	m := core.NewMetric()
	m["test"] = dummyMetric{}

	assert.True(t, m.Success())

	m["test2"] = dummyMetric{status: 1}

	assert.False(t, m.Success())
}

func TestErrorInErrorsEmpty(t *testing.T) {
	res := core.ErrorInErrors(nil, fmt.Errorf("any"))
	assert.False(t, res)
}

func TestErrorInErrors(t *testing.T) {
	const total = 10
	errs := make([]error, total)
	for i := 0; i < total; i++ {
		errs[i] = fmt.Errorf("e%d", i)
	}

	for i := 0; i < total; i++ {
		assert.True(t, core.ErrorInErrors(errs, errs[i]))
	}
}

func TestErrorInErrorsNotFound(t *testing.T) {
	const total = 10
	errs := make([]error, total)
	for i := 0; i < total; i++ {
		errs[i] = fmt.Errorf("e%d", i)
	}

	assert.False(t, core.ErrorInErrors(errs, fmt.Errorf("any")))
	assert.False(t, core.ErrorInErrors(errs, fmt.Errorf("e1")))
}
