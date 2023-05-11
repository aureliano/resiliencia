# resiliencia

[![CI Pipeline](https://github.com/aureliano/resiliencia/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/aureliano/resiliencia/actions/workflows/build.yml?query=branch%3Amain)
[![Coverage](https://coveralls.io/repos/github/aureliano/resiliencia/badge.svg?branch=main)](https://coveralls.io/github/aureliano/resiliencia?branch=main)
[![resiliencia release (latest SemVer)](https://img.shields.io/github/v/release/aureliano/resiliencia?sort=semver)](https://github.com/aureliano/resiliencia/releases)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/aureliano/resiliencia)](https://pkg.go.dev/github.com/aureliano/resiliencia)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Resiliência is a fault tolerance Go library, whose goal is to gather algorithms that implement resiliency patterns.

## Installation
To install Resiliência, use `go get`:

`go get github.com/aureliano/resiliencia`

Or you can install specific version as:

`go get github.com/aureliano/resiliencia/v0`

Or even add it as a project depency of your module:

`require github.com/aureliano/resiliencia v0`

## Policies

This library provides some fault tolerance policies, which can be used singly to wrap a function
or chain other policies together.



|Policy| Premise | Aka| How does the policy mitigate?|
| ------------- | ------------- |:-------------: |------------- |
|**Circuit-breaker**<br/><sub>([example](#))</sub>|When a system is seriously struggling, failing fast is better than making users/callers wait.  <br/><br/>Protecting a faulting system from overload can help it recover. | "Stop doing it if it hurts" <br/><br/>"Give that system a break" | Breaks the circuit (blocks executions) for a period, when faults exceed some pre-configured threshold. |
|**Fallback**<br/><sub>([example](#))</sub>|Things will still fail - plan what you will do when that happens.| "Degrade gracefully"  |Defines an alternative value to be returned (or action to be executed) on failure. |
|**Retry** <br/><sub>([example](#))</sub>|Many faults are transient and may self-correct after a short delay.| "Maybe it's just a blip" |  Allows configuring automatic retries. |
|**Timeout**<br/><sub>([example](#))</sub>|Beyond a certain wait, a success result is unlikely.| "Don't wait forever"  |Guarantees the caller won't have to wait beyond the timeout. |

For individual use of each policy, access the package referring to it to see
its documentation. Below we will see how to use a decorator or a policy chain of responsibility.

### Policy decorator

A decorator allows you to decorate a command with one or more policies. These are chained so that the call to the
service can be made within a circuit breaker, fallback, retry or timeout. In the example below, the command will be
called within the policies in the following order: timeout, retry, circuit breaker and fallback.

```go
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
```

### Policy chain

A policy chain, unlike the decorator, allows you to determine the order in which policies will be chained together.
In this case: retry, circuit breaker, timeout and fallback.

```go
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
```

### Staying up to date
To update Resiliência to the latest version, use `go get -u github.com/aureliano/resiliencia`.

## Contributing
Please feel free to submit issues, fork the repository and send pull requests! But first, read [this guide](./CONTRIBUTING.md) in order to get orientations on how to contribute the best way.

## License
This project is licensed under the terms of the MIT license found in the [LICENSE](./LICENSE) file.
