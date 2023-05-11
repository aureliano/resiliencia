/*
The timeout pattern is pretty straightforward and many HTTP clients have a default timeout configured.
The goal is to avoid unbounded waiting times for responses and thus treating every request as failed
where no response was received within the timeout.

# Usage

There are two ways to run a command under a policy. Using a Command supplier
(anonymous function) or a wrapped policy.

# Command supplier

	p := timeout.New("service-id")
	p.Timeout = time.Second * 45
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

	// Prints Timeout metric.
	fmt.Println(metric)

# Wrapped policy

	policy := new(AnyPolicy)
	policy.Command = func() error {
		// Your business logic.
		...

		// Returns error or nil otherwise.
		return nil
	}

	tm := timeout.New("service-id")
	tm.Timeout = time.Second * 45

	// Instead of a command supplier, it is passed a policy.
	tm.Policy = policy

	metric := core.NewMetric()
	err := tm.Run(metric)

	mr := metric["AnyPolicy"] // or metric[reflect.TypeOf(AnyMetric{}).String()]
	anyMetric, _ := mr.(AnyMetric)

	mr = metric["timeout.Metric"] // or metric[reflect.TypeOf(timeout.Metric{}).String()]
	tmMetric, _ := r.(timeout.Metric)

	// Prints AnyMetric metric.
	fmt.Println(anyMetric)

	// Prints Timeout metric.
	fmt.Println(tmMetric)

# Listener

In order to keep tracking of what is happening on the execution, you may use listeners to generate some events.
timeout supports listeners to track events before and after policy execution.

	p := timeout.New("service-id")
	...
	p.BeforeTimeout = func(p timeout.Policy) {
		fmt.Println("Before timeout.")
	}
	p.AfterTimeout = func(p timeout.Policy, err error) {
		fmt.Println("After timeout.")
	}

	_ = p.Run(core.Metric())
*/
package timeout
