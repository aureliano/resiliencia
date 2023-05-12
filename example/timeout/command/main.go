package main

import (
	"fmt"
	"time"

	"github.com/aureliano/resiliencia/core"
	"github.com/aureliano/resiliencia/timeout"
)

func main() {
	getUsername(1)
	fmt.Printf("\n--------------------------------\n\n")
	getUsername(2)
}

func getUsername(id int) {
	var userName string
	errUserNotFound := fmt.Errorf("user not found")

	policy := timeout.New("service-name")
	policy.Timeout = time.Millisecond * 300
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

	// Timeout error.
	if err != nil {
		fmt.Println("Service call aborted:", err)
	}

	fmt.Println("User name: ", userName)
	fmt.Println("Timeout metric: ", metric["timeout.Metric"])
}

func fetchUserName(id int) string {
	if id == 1 {
		return "resiliencia"
	}

	time.Sleep(time.Millisecond * 350)
	return ""
}
