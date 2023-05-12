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
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(3)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(4)
}

func getUsername(id int) {
	var userName string
	service := "service-name"
	counter := 0

	policy := retry.New(service)
	policy.Tries = 4
	policy.Delay = time.Millisecond * 500
	policy.Errors = []error{errServiceUnavailable}
	policy.Policy = timeout.Policy{
		ServiceID: service,
		Timeout:   time.Millisecond * 100,
		Command: func() error {
			counter++
			name, err := fetchUserName(id, counter)
			if err != nil {
				return err
			}

			userName = name

			return nil
		},
	}

	metric := core.NewMetric()
	err := policy.Run(metric)

	if err != nil {
		fmt.Println("Retry failed:", err)
	}

	fmt.Println("User name: ", userName)
	fmt.Println("Timeout metric: ", metric["timeout.Metric"])
	fmt.Println("Retry metric: ", metric["retry.Metric"])
}

func fetchUserName(id, counter int) (string, error) {
	if id == 1 {
		return "resiliencia", nil
	} else if id == 2 && counter == 3 {
		return "New user", nil
	} else if id == 3 {
		time.Sleep(time.Millisecond * 200)
		return "New user", nil
	}

	return "", errServiceUnavailable
}
