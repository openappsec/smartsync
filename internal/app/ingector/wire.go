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

//go:build wireinject
// +build wireinject

package ingector

import (
	"context"

	"openappsec.io/smartsync-service/internal/app/drivers/eventconsumer"
	"openappsec.io/smartsync-service/internal/app/learning"
	s3repository "openappsec.io/smartsync-service/internal/pkg/db/s3"
	"openappsec.io/smartsync-service/internal/pkg/lock/flock"
	"openappsec.io/smartsync-service/internal/pkg/lock/redis"
	"openappsec.io/kafka/consumermanager"
	redis2 "openappsec.io/redis"

	"openappsec.io/smartsync-service/internal/app"
	"openappsec.io/smartsync-service/internal/app/drivers/http/rest"
	"openappsec.io/health"

	"github.com/google/wire"
	"openappsec.io/configuration"
	"openappsec.io/configuration/viper"
)

// InitializeApp is an Adapter injector
func InitializeApp(ctx context.Context) (*app.App, error) {
	wire.Build(
		viper.NewViper,
		wire.Bind(new(configuration.Repository), new(*viper.Adapter)),

		configuration.NewConfigurationService,
		wire.Bind(new(rest.Configuration), new(*configuration.Service)),
		wire.Bind(new(app.Configuration), new(*configuration.Service)),
		wire.Bind(new(eventconsumer.Configuration), new(*configuration.Service)),
		wire.Bind(new(s3repository.Configuration), new(*configuration.Service)),
		wire.Bind(new(redis.Configuration), new(*configuration.Service)),

		health.NewService,
		wire.Bind(new(rest.HealthService), new(*health.Service)),
		wire.Bind(new(app.HealthService), new(*health.Service)),

		rest.NewAdapter,
		wire.Bind(new(app.RestAdapter), new(*rest.Adapter)),

		s3repository.NewAdapter,
		wire.Bind(new(learning.Repository), new(*s3repository.Adapter)),

		redis2.NewClient,
		wire.Bind(new(redis.Redis), new(*redis2.Adapter)),

		redis.NewAdapter,
		wire.Bind(new(learning.MultiplePodsLock), new(*redis.Adapter)),
		wire.Bind(new(app.DistLock), new(*redis.Adapter)),

		learning.NewLearnService,
		wire.Bind(new(eventconsumer.AppService), new(*learning.LearnCore)),
		wire.Bind(new(rest.AppService), new(*learning.LearnCore)),

		consumermanager.NewConsumerManager,
		wire.Bind(new(eventconsumer.ConsumerManager), new(*consumermanager.ConsumerManager)),

		eventconsumer.NewAdapter,
		wire.Bind(new(app.EventConsumerDriver), new(*eventconsumer.Adapter)),

		app.NewApp,
	)

	return &app.App{}, nil
}

// InitializeStandAloneApp is an Adapter injector
func InitializeStandAloneApp(ctx context.Context) (*app.App, error) {
	wire.Build(
		viper.NewViper,
		wire.Bind(new(configuration.Repository), new(*viper.Adapter)),

		configuration.NewConfigurationService,
		wire.Bind(new(rest.Configuration), new(*configuration.Service)),
		wire.Bind(new(app.Configuration), new(*configuration.Service)),
		wire.Bind(new(s3repository.Configuration), new(*configuration.Service)),
		wire.Bind(new(flock.Configuration), new(*configuration.Service)),

		health.NewService,
		wire.Bind(new(rest.HealthService), new(*health.Service)),
		wire.Bind(new(app.HealthService), new(*health.Service)),

		rest.NewAdapter,
		wire.Bind(new(app.RestAdapter), new(*rest.Adapter)),

		s3repository.NewAdapter,
		wire.Bind(new(learning.Repository), new(*s3repository.Adapter)),

		flock.NewAdapter,
		wire.Bind(new(learning.MultiplePodsLock), new(*flock.Adapter)),
		wire.Bind(new(app.DistLock), new(*flock.Adapter)),

		learning.NewLearnService,
		wire.Bind(new(rest.AppService), new(*learning.LearnCore)),

		app.NewStandAloneApp,
	)

	return &app.App{}, nil
}
