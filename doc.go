/*
Resiliência is a fault tolerance Go library, whose goal is to gather algorithms that implement resiliency patterns.

# Policies

This library provides some fault tolerance policies, which can be used singly to wrap a function
or chain other policies together.

	> Circuit Breaker: When a system is seriously struggling, failing fast is better than making users/callers wait.
	                   Protecting a faulting system from overload can help it recover.
	> Fallback:        Things will still fail - plan what you will do when that happens.
	> Retry:           Many faults are transient and may self-correct after a short delay.
	> Timeout:         Beyond a certain wait, a success result is unlikely.

For individual use of each policy, access the package referring to it to see
its documentation. Below we will see how to use a decorator or a policy chain of responsibility.

# Policy decorator

A decorator allows you to decorate a command with one or more policies. These are chained so that the call to the
service can be made within a circuit breaker, fallback, retry or timeout. In the example below, the command will be
called within the policies in the following order: timeout, retry, circuit breaker and fallback.

	metric, err := resiliencia.
		Decorate(func() error {
			fmt.Println("Do something.")
			return nil
		}).
		WithCircuitBreaker(circuitbreaker.New(id)).
		WithFallback(fallback.Policy{
			ServiceID: id,
			FallBackHandler: func(err error) {
				fmt.Println("Some palliative.")
			},
		}).WithRetry(retry.New(id)).
		WithTimeout(timeout.Policy{
			ServiceID: id,
			Timeout:   time.Minute * 1,
		}).Execute()

# Policy chain

A policy chain, unlike the decorator, allows you to determine the order in which policies will be chained together.
In this case: retry, circuit breaker, timeout and fallback.

	metric, err := resiliencia.Chain(fallback.Policy{
			ServiceID: id,
			FallBackHandler: func(err error) {
				fmt.Println("Some palliative.")
			},
		}, timeout.Policy{
			ServiceID: id,
			Timeout:   time.Minute * 1,
		}, circuitbreaker.New(id), retry.New(id)).
		Execute(func() error {
			fmt.Println("Do something.")
			return nil
		})
*/
package resiliencia
