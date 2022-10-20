// Copyright (C) 2022 Check Point Software Technologies Ltd. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/uber/jaeger-client-go"
	"openappsec.io/errors"
	"openappsec.io/log"
	"openappsec.io/tracer"
)

const (
	retryTopicSuffix = "retry"

	defaultMaxAttempts     = 3
	defaultBackoffDelayMin = 3 * time.Second
	defaultBackoffDelayMax = 10 * time.Second

	minValueMaxAttempts     = 1
	minValueBackoffDelayMin = 1 * time.Second
	minValueBackoffDelayMax = 1 * time.Second

	maxValueMaxAttempts     = 5
	maxValueBackoffDelayMin = time.Minute
	maxValueBackoffDelayMax = time.Minute
)

var retryEnabledNamespaces = []string{"dev-latest", "master-latest", "gem-master-latest", "gem-pre-prod", "pre-prod", "staging", "prod"}

// Consumer is a kafka consumer adapter
type Consumer struct {
	client                 *kafkago.Reader
	brokers                []string
	tls                    bool
	commitAfterHandlingMsg bool
	retryPolicy            *RetryPolicy
	retryProducer          *Producer
}

// NewConsumer returns a kafka consumer implementation
func NewConsumer() *Consumer {
	return &Consumer{}
}

// Initialize initializes the parameters for a connection to kafka server as a consumer
// These configuration fields are required in order to connect:
// brokers, topic, groupId or partition,
func (c *Consumer) Initialize(kafkaConfiguration ConsumerConfig) error {
	span := tracer.GlobalTracer().StartSpan(kafkaOpName)
	defer span.Finish()
	span.SetTag("operation", "connect")

	readerConfig := c.defaultReaderConfig()

	brokers := kafkaConfiguration.Brokers
	if brokers == "" {
		ext.Error.Set(span, true)
		return errors.Errorf("Missing Brokers in kafka configuration")
	}

	readerConfig.Brokers = strings.Split(brokers, ",")

	topic := kafkaConfiguration.Topic
	if topic == "" {
		ext.Error.Set(span, true)
		return errors.Errorf("Missing Topic in kafka configuration")
	}

	readerConfig.Topic = topic

	groupID := kafkaConfiguration.GroupID
	partition := kafkaConfiguration.Partition
	if (groupID == "") == (partition == 0) { // Not XOR
		ext.Error.Set(span, true)
		return errors.Errorf("kafka configuration must contain either GroupID or Partition, and not both!")
	} else if groupID != "" {
		readerConfig.GroupID = groupID
	} else if partition != 0 {
		readerConfig.Partition = partition
	}

	if kafkaConfiguration.Timeout != 0 {
		readerConfig.Dialer.Timeout = kafkaConfiguration.Timeout
	}

	if kafkaConfiguration.MinBytes != 0 {
		readerConfig.MinBytes = kafkaConfiguration.MinBytes
	}

	if kafkaConfiguration.MaxBytes != 0 {
		readerConfig.MaxBytes = kafkaConfiguration.MaxBytes
	}

	c.tls = kafkaConfiguration.TLS
	if !kafkaConfiguration.TLS {
		log.Warn("TLS is disabled for Kafka consumer. This is discouraged for security reasons and can raise issues reading from TLS enabled brokers")
		readerConfig.Dialer.TLS = nil
	}

	// If zero value or set to disabled but namespace should have retry enabled
	namespace := os.Getenv(k8sNamespaceEnvKey)
	for i := range retryEnabledNamespaces {
		if namespace == retryEnabledNamespaces[i] {
			kafkaConfiguration.RetryEnabled = true
			log.Debugf("Current namespace (%s) found in retry enabled namespaces list (%v), enabling retry", namespace, retryEnabledNamespaces)
			break
		}
	}

	if !kafkaConfiguration.RetryEnabled && kafkaConfiguration.RetryPolicy != nil {
		log.Debugf("Disabling RetryPolicy for current namespace (%s) as it's not in retry enabled namespaces list (%v)", namespace, retryEnabledNamespaces)
		kafkaConfiguration.RetryPolicy = nil
	}

	if kafkaConfiguration.RetryPolicy != nil {
		if groupID == "" {
			return errors.Errorf("retry policy is supported only with group Id")
		}
		if err := c.initializeRetryPolicy(kafkaConfiguration); err != nil {
			ext.Error.Set(span, true)
			return errors.Wrap(err, "Failed to initialize retry policy")
		}
	}

	c.commitAfterHandlingMsg = kafkaConfiguration.CommitAfterHandlingMsg
	c.brokers = readerConfig.Brokers
	c.client = kafkago.NewReader(readerConfig)
	c.spanTags(span)

	return nil
}

func (c *Consumer) initializeRetryPolicy(kafkaConfiguration ConsumerConfig) error {
	retryPolicy, err := validateRetryPolicyAndSetDefaults(*kafkaConfiguration.RetryPolicy)
	if err != nil {
		return errors.Wrap(err, "Failed to validate retry policy")
	}
	c.retryPolicy = &retryPolicy
	c.retryProducer = NewProducer()
	originalTopicName := kafkaConfiguration.RetryPolicy.OriginalTopic
	if originalTopicName == "" {
		// on the regular topic (not the retry), save original topic for other retry topics
		// to be able to create topic name of their next retry levels (<originalTopic>__<cg>__retry<#of retry>)
		originalTopicName = kafkaConfiguration.Topic
	}
	retryProducerConfig := ProducerConfig{
		Brokers: kafkaConfiguration.Brokers,
		Topic:   GetRetryTopic(originalTopicName, kafkaConfiguration.GroupID, kafkaConfiguration.RetryPolicy.CurrentAttempt),
		TLS:     kafkaConfiguration.TLS,
	}
	if err := c.retryProducer.Initialize(retryProducerConfig); err != nil {
		return errors.Wrapf(err, "Failed to initialize kafka producer for retry messages (topic = %s)", retryProducerConfig.Topic)
	}

	return nil
}

func validateRetryPolicyAndSetDefaults(retryPolicy RetryPolicy) (RetryPolicy, error) {
	if retryPolicy.DLQTopic == "" {
		return RetryPolicy{}, errors.Errorf("DLQTopic field is required")
	}

	if retryPolicy.MaxAttempts == 0 {
		retryPolicy.MaxAttempts = defaultMaxAttempts
	} else {
		if retryPolicy.MaxAttempts < minValueMaxAttempts {
			return RetryPolicy{}, errors.Errorf("there is a minimum of %d attempt, got %d", minValueMaxAttempts, retryPolicy.MaxAttempts)
		}
		if retryPolicy.MaxAttempts > maxValueMaxAttempts {
			return RetryPolicy{}, errors.Errorf("maximum value for MaxAttempts is %d, got %d", maxValueMaxAttempts, retryPolicy.MaxAttempts)
		}
	}

	if retryPolicy.BackoffDelayMin == 0 {
		retryPolicy.BackoffDelayMin = defaultBackoffDelayMin
	} else {
		if retryPolicy.BackoffDelayMin < minValueBackoffDelayMin {
			return RetryPolicy{}, errors.Errorf("there is a minimum of %s to BackoffDelayMin, got %s", minValueBackoffDelayMin, retryPolicy.BackoffDelayMin)
		}
		if retryPolicy.BackoffDelayMin > maxValueBackoffDelayMin {
			return RetryPolicy{}, errors.Errorf("maximum value for BackoffDelayMin is %s, got %s", maxValueBackoffDelayMin, retryPolicy.BackoffDelayMin)
		}
	}

	if retryPolicy.BackoffDelayMax == 0 {
		retryPolicy.BackoffDelayMax = defaultBackoffDelayMax
	} else {
		if retryPolicy.BackoffDelayMax < minValueBackoffDelayMax {
			return RetryPolicy{}, errors.Errorf("there is a minimum of %s to BackoffDelayMax, got %s", minValueBackoffDelayMax, retryPolicy.BackoffDelayMax)
		}
		if retryPolicy.BackoffDelayMax > maxValueBackoffDelayMax {
			return RetryPolicy{}, errors.Errorf("maximum value for BackoffDelayMax is %s, got %s", maxValueBackoffDelayMax, retryPolicy.BackoffDelayMax)
		}
	}

	if retryPolicy.BackoffDelayMin > retryPolicy.BackoffDelayMax {
		return RetryPolicy{}, errors.Errorf("BackoffDelayMin (%s) can't be greater than BackoffDelayMax (%s)", retryPolicy.BackoffDelayMin, retryPolicy.BackoffDelayMax)
	}

	return retryPolicy, nil
}

// Consume read massage (body and headers) from kafka
// The method call blocks until a message becomes available, or an error occurs.
// The program may also specify a context to asynchronously cancel the blocking operation.
func (c *Consumer) Consume(ctx context.Context) (body []byte, headers map[string][]byte, msg kafkago.Message, err error) {
	cspan, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), kafkaOpName)
	cspan.SetTag("operation", "read message")
	c.spanTags(cspan)

	if c.commitAfterHandlingMsg {
		// FetchMessage reads a message but doesn't commit
		msg, err = c.client.FetchMessage(ctx)
	} else {
		// ReadMessage reads a message and does an immediate commit
		msg, err = c.client.ReadMessage(ctx)
	}

	if err != nil {
		ext.Error.Set(cspan, true)
		cspan.Finish()
		return nil, nil, kafkago.Message{}, err
	}
	cspan.Finish()

	tracerHeaders := make(map[string]string)
	headers = make(map[string][]byte)
	for _, h := range msg.Headers {
		headers[h.Key] = h.Value
		tracerHeaders[h.Key] = string(h.Value)
	}

	// span context can be nil
	spanContext, err := tracer.GlobalTracer().Extract(
		opentracing.TextMap,
		opentracing.TextMapCarrier(tracerHeaders),
	)
	// if there is no parent span (a serviced produced message using kafka client without tracer)
	// the error will be ErrSpanContextNotFound and spanContext will be nil
	if err != opentracing.ErrSpanContextNotFound && err != nil {
		return nil, nil, kafkago.Message{}, errors.Wrap(err, "could not extract span from headers")
	}

	// if spanContext is nil the span will be created as root span and not as 'followFrom'
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), kafkaOpName, opentracing.FollowsFrom(spanContext))
	if sc, ok := span.Context().(jaeger.SpanContext); ok {
		span.SetTag("span.id", sc.SpanID().String())
		span.SetTag("trace.id", sc.TraceID().String())
	}
	defer span.Finish()
	span.SetTag("operation", "follow from message")
	c.spanTags(span)

	return msg.Value, headers, msg, nil
}

// CommitMessage commits a specific message
func (c *Consumer) CommitMessage(ctx context.Context, msg kafkago.Message) error {
	cspan, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), kafkaOpName)
	cspan.SetTag("operation", "commit message")
	c.spanTags(cspan)

	if !c.commitAfterHandlingMsg {
		log.WithContext(ctx).Warn("msg already committed when reading it, ignoring")
		cspan.Finish()
		return nil
	}
	if err := c.client.CommitMessages(ctx, msg); err != nil {
		ext.Error.Set(cspan, true)
		cspan.Finish()
		return err
	}

	cspan.Finish()
	return nil
}

func (c *Consumer) defaultDialer() *kafkago.Dialer {
	timeout, _ := time.ParseDuration(kafkaDefaultDialTimeout)
	return &kafkago.Dialer{
		Timeout:   timeout,
		DualStack: true,
		TLS:       &tls.Config{},
	}
}

func (c *Consumer) defaultReaderConfig() kafkago.ReaderConfig {
	return kafkago.ReaderConfig{
		Dialer:   c.defaultDialer(),
		MinBytes: kafkaDefaultMinBytesPerRequest,
		MaxBytes: kafkaDefaultMaxBytesPerRequest,
	}
}

// HealthCheck checks the health of the mq service
func (c *Consumer) HealthCheck(ctx context.Context) (string, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), kafkaOpName)
	defer span.Finish()
	span.SetTag("operation", "health check")
	c.spanTags(span)

	checkName := "Kafka Consumer Ping Test"
	dialer := c.defaultDialer()
	if !c.tls {
		dialer.TLS = nil
	}

	availableBrokers := 0
	errs := make(map[string]error, len(c.brokers))
	for _, broker := range c.brokers {
		if conn, err := dialer.DialContext(ctx, kafkaDefaultNetworkProtocol, broker); err == nil {
			availableBrokers++
			_ = conn.Close()
		} else {
			errs[broker] = err
		}
	}
	if availableBrokers > 0 {
		if availableBrokers < len(c.brokers) {
			log.WithContext(ctx).Warnf("Cannot reach some of the brokers (%d/%d), errors: %+v",
				availableBrokers, len(c.brokers), errs)
		}
		return checkName, nil
	}
	return checkName, errors.Errorf("Cannot reach all brokers (%d), failing health check (errors: %+v)",
		len(c.brokers), errs)
}

// TearDown disconnect from kafka consumer client
func (c *Consumer) TearDown() error {
	if err := c.client.Close(); err != nil {
		return errors.Wrap(err, "Failed to close kafka consumer connections")
	}

	return nil
}

func (c *Consumer) spanTags(span opentracing.Span) {
	ext.SpanKindConsumer.Set(span)
	ext.MessageBusDestination.Set(span, c.client.Stats().Topic)
	ext.PeerService.Set(span, "kafka")
}

// GetBrokers get the list of all brokers
// this is by ref, when using this do not change values
func (c *Consumer) GetBrokers() []string {
	return c.brokers
}

// GetTopic returns the topic name
func (c *Consumer) GetTopic() string {
	return c.client.Config().Topic
}

// IsCommitAfterHandlingMsg returns true if commitAfterHandlingMsg is true
func (c *Consumer) IsCommitAfterHandlingMsg() bool {
	return c.commitAfterHandlingMsg
}

// GetRetryPolicy returns the retry policy
// this is by ref, when using this do not change values
func (c *Consumer) GetRetryPolicy() *RetryPolicy {
	return c.retryPolicy
}

// GetRetryProducer returns retry producer of the consumer
func (c *Consumer) GetRetryProducer() *Producer {
	return c.retryProducer
}

// GetRetryTopic returns the retry topic name based on topic, group Id and retry level
func GetRetryTopic(topic, groupID string, level int) string {
	return fmt.Sprintf("%s__%s__%s-%s", topic, groupID, retryTopicSuffix, strconv.Itoa(level))
}
