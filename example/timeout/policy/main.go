package main

import (
	"fmt"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/retry"
	"github.com/aureliano/resiliencia/timeout"
)

var errServiceUnavailable = fmt.Errorf("service unavailable")

func main() {
	getUsername(1)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(2)
}

func getUsername(id int) {
	var userName string
	service := "service-name"

	policy := timeout.New("service-name")
	policy.Timeout = time.Second * 5
	policy.Policy = retry.Policy{
		ServiceID: service,
		Tries:     5,
		Delay:     time.Second * 1,
		Errors:    []error{errServiceUnavailable},
		Command: func() error {
			name, err := fetchUserName(id)
			if err != nil {
				return err
			}

			userName = name

			return nil
		},
	}

	metric := core.NewMetric()
	err := policy.Run(metric)

	// Timeout error.
	if err != nil {
		fmt.Println("Service call aborted:", err)
	}

	fmt.Println("User name: ", userName)
	fmt.Println("Retry metric: ", metric["retry.Metric"])
	fmt.Println("Timeout metric: ", metric["timeout.Metric"])
}

func fetchUserName(id int) (string, error) {
	if id == 1 {
		return "resiliencia", nil
	}

	return "", errServiceUnavailable
}
