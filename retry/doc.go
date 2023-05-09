/*
The basis for the retry pattern is to handle transient failures to a service or network resource.
It enables an application to handle transient failures when it tries to connect to a service or network resource,
by transparently retrying a failed operation. Requests temporarily fail for many reasons.
Examples of these failures include the network having a
faulty connection, a site is reloading after a deployment, or data hasn't propagated to all instances.

# Usage

There are two ways to run a command under a policy. Using a Command supplier
(anonymous function) or a wrapped policy.

# Command supplier

	p := retry.New("service-id")
	p.Errors = []error{err1, err2}
	p.Tries = 3
	p.Delay = time.Millisecond * 500
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

	// Prints Retry metric.
	fmt.Println(metric)

# Wrapped policy

	policy := new(AnyPolicy)
	policy.Command = func() error {
		// Your business logic.
		...

		// Returns error or nil otherwise.
		return nil
	}

	rt := retry.New("service-id")
	rt.Errors = []error{err1, err2}
	rt.Tries = 3
	rt.Delay = time.Millisecond * 500

	// Instead of a command supplier, it is passed a policy.
	rt.Policy = policy

	metric := core.NewMetric()
	err := rt.Run(metric)

	mr := metric["AnyPolicy"] // or metric[reflect.TypeOf(AnyMetric{}).String()]
	anyMetric, _ := mr.(AnyMetric)

	mr = metric["retry.Metric"] // or metric[reflect.TypeOf(retry.Metric{}).String()]
	rtMetric, _ := r.(retry.Metric)

	// Prints AnyMetric metric.
	fmt.Println(anyMetric)

	// Prints Retry metric.
	fmt.Println(rtMetric)

# Listener

In order to keep tracking of what is happening on the execution, you may use listeners to generate some events.
retry supports listeners to track events before and after policy execution.

	p := retry.New("service-id")
	...

	// Executes before each execution.
	p.BeforeTry = func(p retry.Policy) {
		fmt.Println("Before retry.")
	}

	// Executes after each execution.
	p.AfterTry = func(p retry.Policy, err error) {
		fmt.Println("After retry.")
	}

	_ = p.Run(core.Metric())
*/
package retry
