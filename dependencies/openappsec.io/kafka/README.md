# Kafka

A package aimed to bring a convenient way to produce to and consume from Kafka. 

## Producer

### NewProducer:

Creates an uninitialized Kafka producer

### Connect:

Initializes a Kafka producer based on the configuration passed to it as a map.<br>
Fields that can be passed in the configuration:

| Field | Value | Description |
|--------------------------------|---------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Kafka Broker | `brokers` | A string containing comma saperated list of the Kafka broker URLs. For example:<br>" b-1.ngen-kafka-cluster.6pue6s.c3.kafka.eu-west-1.amazonaws.com:9094,b-2.ngen-kafka-cluster.6pue6s.c3.kafka.eu-west-1.amazonaws.com:9094,b-3.ngen-kafka-cluster.6pue6s.c3.kafka.eu-west-1.amazonaws.com:9094" |
| Topic | `topic` | The topic to which to produce messages |
| Timeout | `timeout` | The timeout for the produce operation. Must be a valid string representation of `time.Duration` as parsed by [`func ParseDuration(s string) (Duration, error)`](https://golang.org/pkg/time/#ParseDuration) |
| Max number of produce attempts | `maxAttempts` | The maximum number of times a produce operation is attempted before giving up |
| TLS Enabled | `tls` | A boolean value indicating whether the Kafka connection should use TLS |

### Produce:

Accepts a message body (`[]byte`) and headers (`headers map[string][]byte`) and writes the two to the Kafka broker, under the configured topic.<br>
Upon failure an error is returned. <br>
> <b>NOTE:</b> Since this package has built in open-tracing instrumentation, additional, open-tracing, headers are added to the message alongside the provided headers. 

### HealthCheck:

Provides a healthcheck for the Kafka producer. The check is successful if at least one of the configured brokers is reachable.
Note that this health check does not guarantee future `Produce` operations to pass, it only checks for connectivity with the configured brokers.

### TearDown:

Disconnects from the Kafka brokers and closes the client. 

## Consumer

### NewConsumer:

Creates an uninitialized Kafka consumer

### Connect:

Initializes a Kafka consumer based on the configuration passed to it as a map.<br>
Fields that can be passed in the configuration:

| Field | Value | Description |
|---------------|-------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Kafka Broker | `brokers` | A string containing comma saperated list of the Kafka broker URLs. For example:<br>" b-1.ngen-kafka-cluster.6pue6s.c3.kafka.eu-west-1.amazonaws.com:9094,b-2.ngen-kafka-cluster.6pue6s.c3.kafka.eu-west-1.amazonaws.com:9094,b-3.ngen-kafka-cluster.6pue6s.c3.kafka.eu-west-1.amazonaws.com:9094" |
| Topic | `topic` | The topic from which to consume messages |
| Group ID | `groupId` | The ID of the group to which this consumer belongs. Note that consumers in the same group will act according to the queue model where only one consumer instance will receive a given produced message. [Read more about Kafka Consumers](https://kafka.apache.org/documentation/#intro_consumers)  |
| Partition ID | `partition` | The partition from which to consume messages. By using this option, consumer instances will act according to the pub-sub model (see link above). As such, this option is mutually exclusive with `groupId`, only one of these options can be specified. |
| Timeout | `timeout` | The timeout for the consume operation. Must be a valid string representation of `time.Duration` as parsed by [`func ParseDuration(s string) (Duration, error)`](https://golang.org/pkg/time/#ParseDuration) |
| Minimum Bytes | `minBytes` | Minimum number of bytes to fetch from kafka upon consume |
| Maximum Bytes | `maxBytes` | Maximum number of bytes to fetch from kafka upon consume |
| TLS Enabled | `tls` | A boolean value indicating whether the Kafka connection should use TLS |

### Consume:

Consume read massage (body and headers) from kafka.<br>
The method call blocks until a message becomes available, or an error occurs.

### HealthCheck:

Provides a healthcheck for the Kafka consumer. The check is successful if at least one of the configured brokers is reachable.
Note that this health check does not guarantee future `Consume` operations to pass, it only checks for connectivity with the configured brokers.

### TearDown:

Disconnects from the Kafka brokers and closes the client. 


### Multi Consumer

The library also support miltiple consumers. 

To achieve this, create a `ConsumerManager`, using `NewConsumerManager` instead of `NewConsumer`.
See relevant methods below.


### Add
Recieves a `ConsumerConfig` (which includes broker and topic) and a handler function defined: 

`func(ctx context.Context, body []byte, headers map[string][]byte, err error)`. 

Whenever a message is consumed, the function is invoked.
The function initializes the Kafka consumer and adds it to a list.

### Run
Begins the listening process for all added kafka configurations (brokers and topics).
Once Run has been called, no new Kafka configurations may be added.

### HealthCheck:

Provides a healthcheck for all Kafka consumers added.

### TearDown:

Disconnects from the Kafka brokers and closes the client for all consumers added.
