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

// Producer is a kafka producer adapter
type Producer struct {
	client  *kafkago.Writer
	brokers []string
	tls     bool
	topic   string
}

// NewProducer returns a kafka producer implementation
func NewProducer() *Producer {
	return &Producer{}
}

//Initialize initializes the parameters for a connection to kafka server as a producer
// These configuration fields are required in order to connect:
// brokers, topic
func (p *Producer) Initialize(kafkaConfiguration ProducerConfig) error {
	span := tracer.GlobalTracer().StartSpan(kafkaOpName)
	defer span.Finish()
	span.SetTag("operation", "connect")

	writerConfig := p.defaultWriterConfig()

	brokers := kafkaConfiguration.Brokers
	if brokers == "" {
		ext.Error.Set(span, true)
		return errors.Errorf("Missing Brokers in kafka configuration")
	}

	writerConfig.Brokers = strings.Split(brokers, ",")

	topic := kafkaConfiguration.Topic
	if topic == "" {
		ext.Error.Set(span, true)
		return errors.Errorf("Missing Topic in kafka configuration")
	}

	writerConfig.Topic = topic

	if kafkaConfiguration.Timeout != 0 {
		writerConfig.Dialer.Timeout = kafkaConfiguration.Timeout
	}

	if kafkaConfiguration.MaxAttempts != 0 {
		writerConfig.MaxAttempts = kafkaConfiguration.MaxAttempts
	}

	p.tls = kafkaConfiguration.TLS
	if !kafkaConfiguration.TLS {
		log.Warn("TLS is disabled for Kafka producer. This is discouraged for security reasons and can raise issues writing to TLS enabled brokers")
		writerConfig.Dialer.TLS = nil
	}

	p.brokers = writerConfig.Brokers
	p.topic = writerConfig.Topic
	p.client = kafkago.NewWriter(writerConfig)

	p.spanTags(span)

	return nil
}

// Produce writes massage (body and headers) to kafka
func (p *Producer) Produce(ctx context.Context, body []byte, headers map[string][]byte) error {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), kafkaOpName)
	defer span.Finish()
	p.spanTags(span)

	if sc, ok := span.Context().(jaeger.SpanContext); ok {
		span.SetTag("span.id", sc.SpanID().String())
		span.SetTag("trace.id", sc.TraceID().String())
	}

	var hdrs []kafkago.Header
	for key, value := range headers {
		hdrs = append(hdrs, kafkago.Header{Key: key, Value: value})
	}

	tracerHeaders := make(map[string]string)
	if err := tracer.GlobalTracer().Inject(
		span.Context(),
		opentracing.TextMap,
		opentracing.TextMapCarrier(tracerHeaders),
	); err != nil {
		ext.Error.Set(span, true)
		return err
	}

	for key, value := range tracerHeaders {
		hdrs = append(hdrs, kafkago.Header{Key: key, Value: []byte(value)})
	}

	message := kafkago.Message{
		Time:    time.Now().UTC(),
		Headers: hdrs,
		Value:   body,
	}

	return p.client.WriteMessages(ctx, message)
}

func (p *Producer) defaultDialer() *kafkago.Dialer {
	timeout, _ := time.ParseDuration(kafkaDefaultDialTimeout)
	return &kafkago.Dialer{
		Timeout:   timeout,
		DualStack: true,
		TLS:       &tls.Config{},
	}
}

func (p *Producer) defaultWriterConfig() kafkago.WriterConfig {
	return kafkago.WriterConfig{
		Dialer:      p.defaultDialer(),
		Balancer:    &kafkago.LeastBytes{},
		MaxAttempts: kafkaDefaultMaxAttempts,
	}
}

// HealthCheck checks the health of the mq service
func (p *Producer) HealthCheck(ctx context.Context) (string, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), kafkaOpName)
	defer span.Finish()
	span.SetTag("operation", "health check")
	p.spanTags(span)

	checkName := "Kafka Producer Ping Test"
	dialer := p.defaultDialer()
	if !p.tls {
		dialer.TLS = nil
	}
	availableBrokers := 0
	errs := make(map[string]error, len(p.brokers))
	for _, broker := range p.brokers {
		if conn, err := dialer.DialContext(ctx, kafkaDefaultNetworkProtocol, broker); err == nil {
			availableBrokers++
			_ = conn.Close()
		} else {
			errs[broker] = err
		}
	}
	if availableBrokers > 0 {
		if availableBrokers < len(p.brokers) {
			log.WithContext(ctx).Warnf("Could not reach some of the brokers (%d/%d), errors: %+v",
				availableBrokers, len(p.brokers), errs)
		}
		return checkName, nil
	}
	return checkName, errors.Errorf("Could not all brokers (%d), failing health check (errors: %+v)",
		len(p.brokers), errs)
}

// GetTopic returns the topic name
func (p *Producer) GetTopic() string {
	return p.topic
}

// TearDown disconnect from kafka producer client
func (p *Producer) TearDown() error {
	if err := p.client.Close(); err != nil {
		return errors.Wrap(err, "Failed to close kafka producer connections")
	}

	return nil
}

func (p *Producer) spanTags(span opentracing.Span) {
	ext.SpanKindProducer.Set(span)
	ext.MessageBusDestination.Set(span, p.client.Stats().Topic)
	ext.PeerService.Set(span, "kafka")
}
