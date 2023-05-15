package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/fallback"
	"github.com/aureliano/resiliencia/timeout"
)

func main() {
	getUsername(1)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(2)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(3)
}

func getUsername(id int) {
	var userName string
	errUserNotFound := fmt.Errorf("user not found")
	service := "service-name"

	policy := fallback.New(service)
	policy.Errors = []error{errUserNotFound, timeout.ErrExecutionTimedOut} // Comment this line to get a panic error.
	policy.FallBackHandler = func(err error) {
		if errors.Is(err, errUserNotFound) {
			userName = "New user"
		} else if errors.Is(err, timeout.ErrExecutionTimedOut) {
			userName = "Unknown"
		} else {
			fmt.Println("Error:", err)
		}
	}

	policy.Policy = timeout.Policy{
		ServiceID: service,
		Timeout:   time.Millisecond * 200,
		Command: func() error {
			name := fetchUserName(id)
			if name == "" {
				return errUserNotFound
			}

			userName = name

			return nil
		},
	}

	metric := core.NewMetric()
	err := policy.Run(metric)

	// Fallback failed because an unhandlled error.
	if err != nil {
		panic(err)
	}

	fmt.Println("User name: ", userName)
	fmt.Println("Timeout metric: ", metric["timeout.Metric"])
	fmt.Println("Fallback metric: ", metric["fallback.Metric"])
}

func fetchUserName(id int) string {
	if id == 1 {
		return "resiliencia"
	} else if id == 2 {
		time.Sleep(time.Millisecond * 250)
		return "???"
	}

	return ""
}
