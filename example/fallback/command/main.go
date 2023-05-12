package main

import (
	"fmt"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/fallback"
)

func main() {
	getUsername(1)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(2)
}

func getUsername(id int) {
	var userName string
	errUserNotFound := fmt.Errorf("user not found")

	policy := fallback.New("service-name")
	policy.Errors = []error{errUserNotFound} // Comment this line to get a panic error.
	policy.FallBackHandler = func(err error) {
		userName = "New user"
	}
	policy.Command = func() error {
		name := fetchUserName(id)
		if name == "" {
			return errUserNotFound
		}

		userName = name

		return nil
	}

	metric := core.NewMetric()
	err := policy.Run(metric)

	// Fallback failed because an unhandlled error.
	if err != nil {
		panic(err)
	}

	fmt.Println("User name: ", userName)
	fmt.Println("Fallback metric: ", metric["fallback.Metric"])
}

func fetchUserName(id int) string {
	if id == 1 {
		return "resiliencia"
	}

	return ""
}
