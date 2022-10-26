<div align=center>
<img src="https://i2-s3-ui-static-content-prod-10.s3.eu-west-1.amazonaws.com/elpis/tree-no-bg-256.png" width="100" height="100"> 
<h1>openappsec/smartsync</h1>
</div>

## About

open-appsec is a machine learning security engine that preemptively and automatically prevent threats against Web Application & APIs.

open-appsec smartsync service is in charge of correlating learning data from multiple agent instances and delivering a unified learning model for each asset.

## open-appsec smartsync service compilation instructions

### Prequisites

In order to build the service process - golang 1.18 must be deployed on build machine.
Instructions how to install golang can be found here: https://go.dev/doc/install

(The above is not needed when building the service as a container)

### Compiling smartsync service process

1. Clone this repository 
2. Build using golang

```bash
 $ git clone https://github.com/openappsec/smartsync.git
 $ cd smartsync/
 $ go build -o server ./cmd/server/main.go
```

### Building smartsync service container

1. Clone this repository 
2. Build docker image using docker client

```bash
 $ git clone https://github.com/openappsec/smartsync.git
 $ cd smartsync/
 $ docker build -f build/package/Dockerfile . -t smartsync
```

## License
open-appsec/smartsync is open source and available under the Apache 2.0 license.
