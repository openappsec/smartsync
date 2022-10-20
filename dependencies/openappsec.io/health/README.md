# Health
This library serves as a tool for creating liveness and readiness probes for golang service. 

Contained is the infrastructure with which to create healthcehcks and a pair of HTTP endpoints which can be used to expose said checks in a given golang service. 

## Capabilities:
Included are a pair of functions, `Live` and `Ready`, together they define the healthcheck capabilities supplied by this library and exposed by its HTTP handlers.

### Live:
Upon invocation `Live` returns a `LivenessResponse` containing:

| Field | Value |
|-------------|---------------------------------|
| `up` | a constant boolean `true` value<br>(if the function can return a value then the application is surely live) |
| `timestamp` | the current time |

### Ready:
An applications readiness is defined by its `checker`s. 

Checks can be added via the `AddReadinessChecker` method.

Should all checks pass the application is declared as ready. 

Upon invocation `Ready` returns a `ReadinessResponse` containing:

| Field | Value |
|-------------|---------------------------------|
| `ready` | a boolean indicating if the application is ready|
| `uptime` | the application uptime |
| `checkResults` | the result of all readiness checks, indicating which passed and which failed |
| `timestamp` | the current time |

### AddReadinessChecker:
Adds a readiness check to the applications readiness probe. 
A `Check` must implement the following interface:

```go
// Checker is an interface which defines a backend check that returns the check name and an error if it fails
type Checker interface {
	HealthCheck(ctx context.Context) (string, error)
}
```

<b>Example</b>:

A `Redis` HealthCheck which can be used as a `Checker`

```go
// HealthCheck PINGs the Redis server to check if it is accessible and alive
func (a *Adapter) HealthCheck(ctx context.Context) (string, error) {
	checkName := "Redis PING Test"
	log.WithContext(ctx).Infoln("Pinging Redis server...")
	if pong, err := a.client.WithContext(ctx).Ping().Result(); err != nil || pong != "PONG" {
		log.WithContext(ctx).Infof("HealthCheck error while trying to reach Redis: %s", err)
		return checkName, errors.New("Cannot reach Redis, failing health check").SetClass(errors.ClassInternal)
	}

	return checkName, nil
}
```

## HTTP Endpoints:
In addition to the healthCheck infrastructure, this library supplies HTTP endpoints to expose its capabilities.

To use these handlers, a valid `HealthService` must be passed as input. Such a service is one which implements the `Live` and `Ready` methods mentioned above. 

Conveniently, this library contains such an implementation. See usage example below.

### LivenessHandler:
Accepts a `HealthService` and returns an HTTP handler exposing the `Live` operation.

### ReadinessHandler:
Accepts a `HealthService` and returns an HTTP handler exposing the `Ready` operation.

### Usage Example:

```go
package main

import (
	"context"
	"log"
	"net/http"

	"openappsec.io/health"
	"openappsec.io/health/http/rest"

	"github.com/gorilla/mux"
)

type example struct {
}

func (a *example) HealthCheck(ctx context.Context) (string, error) {
	return "Example Health Test", nil
}

func main() {
	healthService := health.NewService()
	e := example{}
	healthService.AddReadinessChecker(&e)

	router := mux.NewRouter()
	router.HandleFunc("/live", rest.LivenessHandler(healthService).ServeHTTP).Methods(http.MethodGet)
	router.HandleFunc("/ready", rest.ReadinessHandler(healthService).ServeHTTP).Methods(http.MethodGet)
	log.Fatal(http.ListenAndServe(":8080", router))
}
```


