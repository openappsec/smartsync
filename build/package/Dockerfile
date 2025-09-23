############################
# STEP 1 build executable binary
############################
# golang alpine 1.18
FROM golang:tip-alpine3.22 as builder

RUN mkdir /app
WORKDIR /app

COPY . .

# update path to dependencies.
ENV GOMODCACHE /vendor

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -a -installsuffix cgo -o /go/bin/app ./cmd/server/

############################
# STEP 2 build a small image
############################
# alpine linux 3.13.6
FROM alpine:3.22.1

# Copy our static executable
COPY --from=builder /go/bin/app /go/bin/app

# Copy configuration files
COPY --from=builder /app/configs/* ./configs/

# Create appuser so as not to run as root
RUN adduser -D -g '' appuser

RUN apk add libcap
RUN setcap CAP_NET_BIND_SERVICE=+eip /go/bin/app

# Use an unprivileged user.
USER appuser

ENV APPSEC_MODE "standalone"
# Port on which the service will be exposed.
EXPOSE 80
EXPOSE 8080

# Run the app binary.
ENTRYPOINT ["/go/bin/app"]
