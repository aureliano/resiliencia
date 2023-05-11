/*
The circuitbreaker package contains the implementation of the circuit breaker pattern. This pattern helps
to avoid cascading failures and to build a fault-tolerant and resilient service. So that it can survive
when the services it consumes are unavailable or with high latency.

# Usage

There are two ways to run a command under a policy. Using a Command supplier
(anonymous function) or a wrapped policy.

# Command supplier

	p := circuitbreaker.New("service-id")
	p.ThresholdErrors = 5
	p.ResetTimeout = time.Second * 25
	p.Command = func() error {
		// Your business logic.
		...

		// Returns error or nil otherwise.
		return nil
	}

	metric := core.NewMetric()
	err := p.Run(metric)

	if err != nil {
		// Error handling.
		...
	}

	// Prints Circuit Breaker metric.
	fmt.Println(metric)

# Wrapped policy

	policy := new(AnyPolicy)
	policy.Command = func() error {
		// Your business logic.
		...

		// Returns error or nil otherwise.
		return nil
	}

	cb := circuitbreaker.New("service-id")
	cb.ThresholdErrors = 5
	cb.ResetTimeout = time.Second * 25

	// Instead of a command supplier, it is passed a policy.
	cb.Policy = policy

	metric := core.NewMetric()
	err := cb.Run(metric)

	mr := metric["AnyPolicy"] // or metric[reflect.TypeOf(AnyMetric{}).String()]
	anyMetric, _ := mr.(AnyMetric)

	mr = metric["circuitbreaker.Metric"] // or metric[reflect.TypeOf(circuitbreaker.Metric{}).String()]
	cbMetric, _ := r.(circuitbreaker.Metric)

	// Prints AnyMetric metric.
	fmt.Println(anyMetric)

	// Prints Circuit Breaker metric.
	fmt.Println(cbMetric)

# Listener

In order to keep tracking of what is happening on the execution, you may use listeners to generate some events.
Circuit Breaker supports listeners to track some events that are described bellow.

	p := circuitbreaker.New("service-id")
	...

	// It is called before any execution.
	p.BeforeCircuitBreaker = func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {
		fmt.Println("Before circuit breaker.")
	}

	// It is called only when the circuit breaker changes to open.
	p.OnOpenCircuit = func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {
		fmt.Println("Circuit breaker has just open.")
	}

	// It is called only when the circuit breaker changes to half open.
	p.OnHalfOpenCircuit = func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {
		fmt.Println("Circuit breaker has just half open.")
	}

	// It is called only when the circuit breaker changes to closed.
	p.OnClosedCircuit = func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker) {
		fmt.Println("Circuit breaker has just closed.")
	}

	// It is called after any execution.
	p.AfterCircuitBreaker = func(p circuitbreaker.Policy, status *circuitbreaker.CircuitBreaker, err error) {
		fmt.Println("After circuit breaker.")
	}

	_ = p.Run(core.Metric())
*/
package circuitbreaker
