package main

import (
	"fmt"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/retry"
)

var errServiceUnavailable = fmt.Errorf("service unavailable")

func main() {
	getUsername(1)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(2)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(3)
}

func getUsername(id int) {
	var userName string
	service := "service-name"
	counter := 0

	policy := retry.New(service)
	policy.Tries = 3
	policy.Delay = time.Millisecond * 500
	policy.Errors = []error{errServiceUnavailable}
	policy.Command = func() error {
		counter++
		name, err := fetchUserName(id, counter)
		if err != nil {
			return err
		}

		userName = name

		return nil
	}

	metric := core.NewMetric()
	err := policy.Run(metric)

	if err != nil {
		fmt.Println("Retry failed:", err)
	}

	fmt.Println("User name: ", userName)
	fmt.Println("Retry metric: ", metric["retry.Metric"])
}

func fetchUserName(id, counter int) (string, error) {
	if id == 1 {
		return "resiliencia", nil
	} else if id == 2 && counter == 3 {
		return "New user", nil
	}

	return "", errServiceUnavailable
}
