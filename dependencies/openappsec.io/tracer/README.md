# Jaeger

![jaeger Logo](https://openappsec.io/jaeger/raw/dev/assets/logo.png)

[![jaeger status](https://openappsec.io/jaeger/badges/dev/pipeline.svg)](https://openappsec.io/jaeger/commits/dev) [![coverage report](https://openappsec.io/jaeger/badges/dev/coverage.svg)](https://openappsec.io/jaeger/commits/dev)


## General
This package provide a simple SDK for opentracing API 
it provides an API with the following capabilities:
* Initialize a new tracer
* Get the global tracer  


## Important Note:
Tracer initialize a no-op tracer on startup which
No-op tracer is a trivial, minimum overhead implementation of Tracer
for which all operations are no-ops.

## Initialization

```
import (
    "openappsec.io/tracer"
)

func main() {
	err := tracer.InitGlobalTracer("your-service-name", "tracer-host:port")
}
```  
  