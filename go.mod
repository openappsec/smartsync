module openappsec.io/smartsync-service

go 1.18

require (
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/google/uuid v1.1.2
	github.com/google/wire v0.5.0
	github.com/hashicorp/go-uuid v1.0.1
	openappsec.io/configuration v0.5.5
	openappsec.io/ctxutils v0.5.0
	openappsec.io/errors v0.7.0
	openappsec.io/health v0.2.1
	openappsec.io/httputils v0.11.1
	openappsec.io/kafka v0.7.2
	openappsec.io/log v0.9.0
	openappsec.io/redis v0.12.3
	openappsec.io/tracer v0.5.0
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/klauspost/compress v1.15.2 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/segmentio/kafka-go v0.4.17 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.9.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/sys v0.0.0-20220704084225-05e143d24a9e // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/xerrors v0.0.0-20220411194840-2f41105eb62f // indirect
	gopkg.in/ini.v1 v1.64.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace (
	openappsec.io/configuration => ./dependencies/openappsec.io/configuration
	openappsec.io/ctxutils => ./dependencies/openappsec.io/ctxutils
	openappsec.io/errors => ./dependencies/openappsec.io/errors
	openappsec.io/health => ./dependencies/openappsec.io/health
	openappsec.io/httputils => ./dependencies/openappsec.io/httputils
	openappsec.io/kafka => ./dependencies/openappsec.io/kafka
	openappsec.io/log => ./dependencies/openappsec.io/log
	openappsec.io/redis => ./dependencies/openappsec.io/redis
	openappsec.io/tracer => ./dependencies/openappsec.io/tracer
)
