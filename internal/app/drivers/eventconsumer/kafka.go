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

package eventconsumer

import (
	"context"
	"fmt"

	"openappsec.io/smartsync-service/models"

	"openappsec.io/log"

	"openappsec.io/ctxutils"
	"openappsec.io/errors"
	"openappsec.io/kafka"

	"github.com/google/uuid"
)

const (
	k8sNamespace                 = "K8S_NAMESPACE"
	kafkaConfBaseKey             = "kafka"
	kafkaConsumerConfKey         = kafkaConfBaseKey + ".consumer"
	kafkaConsumerTLSConfKey      = kafkaConsumerConfKey + ".tls"
	kafkaConsumerBrokersConfKey  = kafkaConsumerConfKey + ".brokers"
	kafkaConsumerDLQTopicConfKey = kafkaConsumerConfKey + ".dlq.topic"

	learnNotificationConfBaseKey            = kafkaConsumerConfKey + ".learnNotification"
	learnNotificationConsumerTopicConfKey   = learnNotificationConfBaseKey + ".topic"
	learnNotificationConsumerGroupIDConfKey = learnNotificationConfBaseKey + ".groupId"

	eventTraceIDKey = "eventTraceId"
)

// mockgen -destination mocks/mock_consumerManager.go -package mocks openappsec.io/smartsync-service/internal/app/drivers/eventconsumer ConsumerManager

// ConsumerManager exposes the interface for managing kafka consumers
type ConsumerManager interface {
	Add(ctx context.Context, kafkaConfiguration kafka.ConsumerConfig, f kafka.HandleFunc) error
	Run(ctx context.Context)
	HealthCheck(ctx context.Context) (string, error)
	TearDown() error
}

// Configuration exposes an interface of configuration related actions
type Configuration interface {
	GetString(key string) (string, error)
	GetBool(key string) (bool, error)
}

//mockgen -destination mocks/mock_appService.go -package mocks openappsec.io/smartsync-service/internal/app/drivers/eventconsumer AppService

// AppService exposes the domain interface for handling events
type AppService interface {
	ProcessSyncRequest(ctx context.Context, ids models.SyncID) error
}

// Adapter kafka adapter
type Adapter struct {
	cm  ConsumerManager
	srv AppService
}

// NewAdapter creates and register consumer for kafka events
func NewAdapter(cm ConsumerManager, srv AppService, conf Configuration) (*Adapter, error) {
	brokers, err := conf.GetString(kafkaConsumerBrokersConfKey)
	if err != nil {
		return nil, err
	}

	tls, err := conf.GetBool(kafkaConsumerTLSConfKey)
	if err != nil {
		return nil, err
	}

	dlqTopic, err := conf.GetString(kafkaConsumerDLQTopicConfKey)
	if err != nil {
		return nil, err
	}

	topicPrefix, err := conf.GetString(k8sNamespace)
	if err != nil {
		log.Warnf("failed to get k8s namespace")
		topicPrefix = ""
	} else {
		topicPrefix += "_"
	}

	learnNotificationTopic, err := conf.GetString(learnNotificationConsumerTopicConfKey)
	if err != nil {
		return nil, err
	}

	learnNotificationGroupID, err := conf.GetString(learnNotificationConsumerGroupIDConfKey)
	if err != nil {
		return nil, err
	}

	learnNotificationConfig := kafka.ConsumerConfig{
		Brokers: brokers,
		TLS:     tls,
		Topic:   topicPrefix + learnNotificationTopic,
		GroupID: fmt.Sprintf("%s-%s", topicPrefix, learnNotificationGroupID),
		RetryPolicy: &kafka.RetryPolicy{
			MaxAttempts: 5,
			DLQTopic:    dlqTopic,
		},
		CommitAfterHandlingMsg: true,
	}

	a := &Adapter{cm: cm, srv: srv}

	if err := cm.Add(context.Background(), learnNotificationConfig, a.handleNotification); err != nil {
		return nil, errors.Wrap(err, "Failed to add learning sync consumer")
	}

	return a, nil
}

func (a *Adapter) headersToContextMiddleware(ctx context.Context, headers map[string][]byte) context.Context {
	var eventTraceID string
	rawEventTraceID, ok := headers[eventTraceIDKey]
	if ok {
		eventTraceID = string(rawEventTraceID)
	} else {
		eventTraceID = uuid.New().String()
	}

	return ctxutils.Insert(ctx, ctxutils.ContextKeyEventTraceID, eventTraceID)
}

// HealthCheck checks adapter health
func (a *Adapter) HealthCheck(ctx context.Context) (string, error) {
	return a.cm.HealthCheck(ctx)
}

// Start running the consumers
func (a *Adapter) Start(ctx context.Context) {
	log.WithContext(ctx).Infof("event consumer is running")
	a.cm.Run(ctx)
}

// Stop stopping the consumers
func (a *Adapter) Stop(ctx context.Context) error {
	log.WithContext(ctx).Infof("stopping event consumer")
	if err := a.cm.TearDown(); err != nil {
		return errors.Wrap(err, "Failed to gracefully stop message queues")
	}

	return nil
}
