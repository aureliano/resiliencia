package main

import (
	"fmt"
	"time"

	"github.com/aureliano/resiliencia/circuitbreaker"
	"github.com/aureliano/resiliencia/core"
)

var errServiceUnavailable = fmt.Errorf("service unavailable")

func main() {
	getUsername(1)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(2)
	time.Sleep(time.Millisecond * 500)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(1)
}

func getUsername(id int) {
	var userName string
	service := "service-name"

	policy := circuitbreaker.New(service)
	policy.ThresholdErrors = 2
	policy.ResetTimeout = time.Millisecond * 500
	policy.Errors = []error{errServiceUnavailable}
	policy.Command = func() error {
		name, err := fetchUserName(id)
		if err != nil {
			return err
		}

		userName = name

		return nil
	}

	state, _ := circuitbreaker.State(policy)
	fmt.Println("Circuit Breaker state: ", state)

	metric := core.NewMetric()
	err := policy.Run(metric)

	if err != nil {
		fmt.Println("Circuit Breaker failed:", err)
	}

	fmt.Println("User name: ", userName)
	state, _ = circuitbreaker.State(policy)
	fmt.Println("Circuit Breaker state: ", state)
	fmt.Println("Circuit Breaker metric: ", metric["circuitbreaker.Metric"])

	metric = core.NewMetric()
	err = policy.Run(metric)

	if err != nil {
		fmt.Println("Circuit Breaker failed:", err)
	}

	fmt.Println("User name: ", userName)
	state, _ = circuitbreaker.State(policy)
	fmt.Println("Circuit Breaker state: ", state)
	fmt.Println("Circuit Breaker metric: ", metric["circuitbreaker.Metric"])
}

func fetchUserName(id int) (string, error) {
	if id == 1 {
		return "resiliencia", nil
	}

	return "", errServiceUnavailable
}
