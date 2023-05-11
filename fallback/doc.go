/*
The Fallback Pattern consists of detecting a problem and then executing on an alternative code path.
This pattern is used when the original code path fails and provides a mechanism that will allow the
service client to respond through alternative means. Other paths may include static responses,
stubbed fallbacks, cached responses, or even alternative services that provide similar information.
Once a failure is detected, perhaps through one of the other resiliency patterns, the system can fallback.

# Usage

There are two ways to run a command under a policy. Using a Command supplier
(anonymous function) or a wrapped policy.

# Command supplier

	p := fallback.New("service-id")
	p.Errors = []error{err1, err2}
	p.FallBackHandler = func(err error) {
		// Do something about that incoming error.
	}
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

	// Prints Fallback metric.
	fmt.Println(metric)

# Wrapped policy

	policy := new(AnyPolicy)
	policy.Command = func() error {
		// Your business logic.
		...

		// Returns error or nil otherwise.
		return nil
	}

	fb := fallback.New("service-id")
	fb.Errors = []error{err1, err2}
	fb.FallBackHandler = func(err error) {
		// Do something about that incoming error.
	}

	// Instead of a command supplier, it is passed a policy.
	fb.Policy = policy

	metric := core.NewMetric()
	err := fb.Run(metric)

	mr := metric["AnyPolicy"] // or metric[reflect.TypeOf(AnyMetric{}).String()]
	anyMetric, _ := mr.(AnyMetric)

	mr = metric["fallback.Metric"] // or metric[reflect.TypeOf(fallback.Metric{}).String()]
	fbMetric, _ := r.(fallback.Metric)

	// Prints AnyMetric metric.
	fmt.Println(anyMetric)

	// Prints Fallback metric.
	fmt.Println(fbMetric)

# Listener

In order to keep tracking of what is happening on the execution, you may use listeners to generate some events.
Fallback supports listeners to track events before and after policy execution.

	p := fallback.New("service-id")
	...
	p.BeforeFallBack = func(p fallback.Policy) {
		fmt.Println("Before fallback.")
	}
	p.AfterFallBack = func(p fallback.Policy, err error) {
		fmt.Println("After fallback.")
	}

	_ = p.Run(core.Metric())
*/
package fallback
