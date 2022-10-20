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

package consumermanager

import (
	"context"
	"time"

	"openappsec.io/kafka"

	"openappsec.io/errors"
	"openappsec.io/log"
)

const (
	dlqTopicHeaderKeyOriginalTopic = "originalTopic"
)

type consumerAndFunc struct {
	consumer *kafka.Consumer
	function kafka.HandleFunc
}

// ConsumerManager a list of all the consumer-func pair
type ConsumerManager struct {
	conAndFunc  []consumerAndFunc
	dlqProducer *kafka.Producer
}

//NewConsumerManager creates a new multi topic consumer
func NewConsumerManager() *ConsumerManager {
	return &ConsumerManager{}
}

//Add adds a config to the list of topics to run on
func (cm *ConsumerManager) Add(ctx context.Context, kafkaConfiguration kafka.ConsumerConfig, f kafka.HandleFunc) error {
	c := kafka.NewConsumer()
	err := c.Initialize(kafkaConfiguration)
	if err != nil {
		return errors.Wrapf(err, "Could not init for config: %+v", kafkaConfiguration)
	}
	cm.conAndFunc = append(cm.conAndFunc, consumerAndFunc{
		consumer: c,
		function: f,
	})
	retryPolicy := c.GetRetryPolicy()
	if retryPolicy != nil {
		// add retry consumers
		// sending retryPolicy separately because it could be modified during initialization (when using default values)
		if err := cm.initRetryConsumers(retryPolicy, kafkaConfiguration, f); err != nil {
			return errors.Wrap(err, "Could not init retry consumers")
		}

		// init dlq only in first time `Add` is being called (there is an assumption that all consumers have the same DLQ)
		if cm.dlqProducer == nil {
			if err := cm.initDLQProducer(kafkaConfiguration); err != nil {
				return errors.Wrap(err, "Could not init DLQ producer")
			}
		}
	}

	return nil
}

//Run starts the topics consume and handles
func (cm *ConsumerManager) Run(ctx context.Context) {
	for _, caf := range cm.conAndFunc {
		go func(caf consumerAndFunc) {
			for {
				body, headers, msg, err := caf.consumer.Consume(ctx)
				if ctx.Err() == context.Canceled {
					break
				}

				// when failed to consume - ignore message
				if err != nil {
					log.WithContext(ctx).Errorf("Failed to consume message (topic: %s). Error: %s", caf.consumer.GetTopic(), err)
					continue
				}

				// for error topic - do a sleep before handling message
				retryPolicy := caf.consumer.GetRetryPolicy()
				if retryPolicy != nil && retryPolicy.CurrentAttempt > 0 {
					if !sleep(ctx, getSleepTime(msg.Time, retryPolicy.CurrentAttempt, retryPolicy.BackoffDelayMin, retryPolicy.BackoffDelayMax)) {
						break
					}
				}

				// handle message
				err = caf.function(ctx, body, headers)

				// when got an internal error - do a retry or publish to DLQ
				if err != nil && errors.IsClassTopLevel(err, errors.ClassInternal) {
					if retryPolicy != nil {
						if hasMoreAttempts(retryPolicy.CurrentAttempt, retryPolicy.MaxAttempts) {
							if err := cm.produceRetryMessage(ctx, caf.consumer.GetRetryProducer(), body, headers); err != nil {
								log.WithContext(ctx).Errorf("Failed to produce kafka retry message (topic: %s): %s", caf.consumer.GetRetryProducer().GetTopic(), err)
							}
						} else {
							if err := cm.produceDLQMessage(ctx, caf.consumer.GetRetryPolicy().OriginalTopic, body, headers); err != nil {
								log.WithContext(ctx).Errorf("Failed to produce kafka message to DLQ: %s", err)
							}
						}
					}
				}

				// do a commit if configured to do it after message handling is done
				if caf.consumer.IsCommitAfterHandlingMsg() {
					if err := caf.consumer.CommitMessage(ctx, msg); err != nil {
						log.WithContext(ctx).Errorf("Failed to commit message (%+v): %s", msg, err)
					}
				}
			}
		}(caf)
	}
}

func (cm *ConsumerManager) initDLQProducer(kafkaConfiguration kafka.ConsumerConfig) error {
	cm.dlqProducer = kafka.NewProducer()
	errorProducerConfig := kafka.ProducerConfig{
		Brokers: kafkaConfiguration.Brokers,
		Topic:   kafkaConfiguration.RetryPolicy.DLQTopic,
		TLS:     kafkaConfiguration.TLS,
	}
	if err := cm.dlqProducer.Initialize(errorProducerConfig); err != nil {
		return errors.Wrap(err, "Failed to initialize kafka producer for DLQ messages")
	}
	return nil
}

func (cm *ConsumerManager) initRetryConsumers(retryPolicy *kafka.RetryPolicy, kafkaConfiguration kafka.ConsumerConfig, f kafka.HandleFunc) error {
	for retry := 0; retry < retryPolicy.MaxAttempts-1; retry++ {
		kafkaRetryConfig := getKafkaRetryConsumerConfig(kafkaConfiguration, *retryPolicy, retry)
		cRetry := kafka.NewConsumer()
		err := cRetry.Initialize(kafkaRetryConfig)
		if err != nil {
			return errors.Wrapf(err, "Could not init for retry consumer (topic: %s)", kafkaRetryConfig.Topic)
		}
		cm.conAndFunc = append(cm.conAndFunc, consumerAndFunc{
			consumer: cRetry,
			function: f,
		})
	}

	return nil
}

func getKafkaRetryConsumerConfig(originalConfig kafka.ConsumerConfig, originalRetryPolicy kafka.RetryPolicy, retry int) kafka.ConsumerConfig {
	return kafka.ConsumerConfig{
		Brokers: originalConfig.Brokers,
		Topic:   kafka.GetRetryTopic(originalConfig.Topic, originalConfig.GroupID, retry),
		// Partition:    0, TODO!!!! INXT-23636
		GroupID:                originalConfig.GroupID,
		Timeout:                originalConfig.Timeout,
		MinBytes:               originalConfig.MinBytes,
		MaxBytes:               originalConfig.MaxBytes,
		TLS:                    originalConfig.TLS,
		CommitAfterHandlingMsg: true,
		RetryPolicy: &kafka.RetryPolicy{
			MaxAttempts:     originalRetryPolicy.MaxAttempts,
			BackoffDelayMin: originalRetryPolicy.BackoffDelayMin,
			BackoffDelayMax: originalRetryPolicy.BackoffDelayMax,
			DLQTopic:        originalRetryPolicy.DLQTopic,
			CurrentAttempt:  retry + 1,
			OriginalTopic:   originalConfig.Topic,
		},
		RetryEnabled: originalConfig.RetryEnabled,
	}
}

func getSleepTime(timestamp time.Time, attempt int, min time.Duration, max time.Duration) time.Duration {
	requiredSleepTime := time.Duration(attempt*attempt) * min
	if requiredSleepTime > max {
		requiredSleepTime = max
	}
	now := time.Now().UTC()
	if timestamp.Add(requiredSleepTime).After(now) {
		// when timestamp + sleep time > current time
		// then no sleep needed
		return 0
	}
	// to get the remaining time to sleep,
	// we subtract the the time that already passed (from timestamp until now) from sleep time
	passedTime := now.Sub(timestamp)
	res := requiredSleepTime - passedTime
	if res < 0 {
		return 0
	}
	return res
}

func sleep(ctx context.Context, duration time.Duration) bool {
	log.WithContext(ctx).Warnf("Waiting %s before handling kafka retry message", duration.String())
	if duration == 0 {
		return true
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func hasMoreAttempts(currentAttempt, maxAttempts int) bool {
	if currentAttempt < maxAttempts-1 {
		return true
	}
	return false
}

func (cm *ConsumerManager) produceRetryMessage(ctx context.Context, producer *kafka.Producer, body []byte, headers map[string][]byte) error {
	log.WithContext(ctx).Debugf("Sending message to retry topic (topic = %s, headers = %+v, body = %s)", producer.GetTopic(), headers, body)
	return producer.Produce(ctx, body, headers)
}

func (cm *ConsumerManager) produceDLQMessage(ctx context.Context, originalTopic string, body []byte, headers map[string][]byte) error {
	log.WithContext(ctx).Errorf(
		"Sending message to DLQ (DLQ topic = %s, original topic = %s, headers = %+v, body = %s)",
		cm.dlqProducer.GetTopic(), originalTopic, headers, body)
	headers[dlqTopicHeaderKeyOriginalTopic] = []byte(originalTopic)
	return cm.dlqProducer.Produce(ctx, body, headers)
}

// HealthCheck checks the health of the mq service
func (cm *ConsumerManager) HealthCheck(ctx context.Context) (string, error) {
	checkName := "Kafka Consumer Ping Test for all consumers"
	brokers := map[string]*kafka.Consumer{}
	for _, c := range cm.conAndFunc {
		for _, b := range c.consumer.GetBrokers() {
			brokers[b] = c.consumer
		}
	}
	var errs []error
	for _, c := range brokers {
		_, err := c.HealthCheck(ctx)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return checkName, errors.Errorf("health check failed for consumers. Errors: %+v", errs)
	}
	return checkName, nil
}

// TearDown disconnect from kafka consumer clients
func (cm *ConsumerManager) TearDown() error {
	errs := make([]error, 0, len(cm.conAndFunc))
	for _, c := range cm.conAndFunc {
		if err := c.consumer.TearDown(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errors.Errorf("Failed to close all kafka Consumer Manager connections: %+v", errs)
	}

	return nil
}
