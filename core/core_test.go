package core_test

import (
	"fmt"
	"testing"

	"github.com/aureliano/resiliencia/core"
	"github.com/stretchr/testify/assert"
)

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
