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

package app

import (
	"context"
	"os"
	"os/signal"
	"time"

	"openappsec.io/errors"
	"openappsec.io/health"
	"openappsec.io/log"
	"openappsec.io/tracer"
)

const (
	appName = "smartsync-service"

	// configuration keys
	logConfBaseKey       = "log"
	logLevelConfKey      = logConfBaseKey + ".level"
	tracerConfBaseKey    = "tracer"
	tracerHostConfKey    = tracerConfBaseKey + ".host"
	tracerEnabledConfKey = tracerConfBaseKey + ".enabled"
)

// RestAdapter defines a driving adapter interface.
type RestAdapter interface {
	Start() error
	Stop(ctx context.Context) error
}

// Configuration exposes an interface of configuration related actions
type Configuration interface {
	SetMany(ctx context.Context, conf map[string]interface{}) error
	Set(ctx context.Context, key string, value interface{}) error
	Get(key string) interface{}
	GetString(key string) (string, error)
	GetInt(key string) (int, error)
	GetBool(key string) (bool, error)
	GetDuration(key string) (time.Duration, error)
	GetAll() map[string]interface{}
	IsSet(key string) bool
	RegisterHook(key string, hook func(value interface{}) error)
	HealthCheck(ctx context.Context) (string, error)
}

// HealthService defines a health domain service
type HealthService interface {
	AddReadinessChecker(checker health.Checker)
}

// EventConsumerDriver defines an event consumer driving adapter interface
type EventConsumerDriver interface {
	Start(ctx context.Context)
	Stop(ctx context.Context) error
	HealthCheck(ctx context.Context) (string, error)
}

// DistLock defines interface for a distributed lock driven adapter
type DistLock interface {
	TearDown(ctx context.Context) error
}

// App defines the application struct.
type App struct {
	httpDriver RestAdapter
	ecDriver   EventConsumerDriver
	conf       Configuration
	health     HealthService
	distLock   DistLock
}

// NewApp returns a new instance of the App.
func NewApp(adapter RestAdapter, conf Configuration, healthSvc HealthService, ec EventConsumerDriver, lock DistLock) *App {
	return &App{
		httpDriver: adapter,
		ecDriver:   ec,
		conf:       conf,
		health:     healthSvc,
		distLock:   lock,
	}
}

// NewStandAloneApp returns a new instance of the App with standalone limits.
func NewStandAloneApp(adapter RestAdapter, conf Configuration, healthSvc HealthService, lock DistLock) *App {
	return &App{
		httpDriver: adapter,
		ecDriver:   nil,
		conf:       conf,
		health:     healthSvc,
		distLock:   lock,
	}
}

// Start begins the flow of the app
func (a *App) Start() error {

	if err := a.loggerInit(); err != nil {
		return errors.Wrap(err, "Failed to initialize logger")
	}

	if err := a.tracerInit(); err != nil {
		return errors.Wrap(err, "Failed to initialize tracer")
	}

	a.healthInit()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c := <-signalChan
		log.WithContext(ctx).Infof("Received signal: %+v", c)
		cancel()
	}()

	return a.serveDriver(ctx)
}

func (a *App) serveDriver(ctx context.Context) error {
	serverStartErrorChan := make(chan error, 1)
	go func() {
		if err := a.httpDriver.Start(); err != nil {
			serverStartErrorChan <- err
		}
	}()

	if a.ecDriver != nil {
		go a.ecDriver.Start(ctx)
	}

	select {
	case <-ctx.Done():
		// graceful shutdown flow, return no error
		// main.go will call Stop()
		return nil
	case err := <-serverStartErrorChan:
		return errors.Wrap(err, "Failed to start server")
	}
}

// Stop does a graceful shutdown of the app
func (a *App) Stop(ctx context.Context) []error {
	var stopErrs []error

	if err := a.httpDriver.Stop(ctx); err != nil {
		stopErrs = append(stopErrs, errors.Errorf("Failed to gracefully shutdown server. Errors: %v", err))
	}

	if a.ecDriver != nil {
		if err := a.ecDriver.Stop(ctx); err != nil {
			stopErrs = append(
				stopErrs,
				errors.Errorf("Failed to gracefully stop event consumer driver. Errors: %v", err),
			)
		}
	}

	if err := a.distLock.TearDown(ctx); err != nil {
		stopErrs = append(
			stopErrs,
			errors.Errorf("Failed to gracefully tear down distributed lock. Errors: %v", err),
		)
	}

	return stopErrs
}

func (a *App) healthInit() {
	if a.ecDriver != nil {
		a.health.AddReadinessChecker(a.ecDriver)
	}
}

func (a *App) loggerInit() error {
	if err := a.setLogLevelFromConfiguration(); err != nil {
		return errors.Wrap(err, "Failed to set log level from configuration")
	}
	a.conf.RegisterHook(
		logLevelConfKey, func(value interface{}) error {
			if err := a.setLogLevelFromConfiguration(); err != nil {
				return errors.Wrap(err, "Failed to set log level from configuration")
			}

			return nil
		},
	)

	return nil
}

func (a *App) setLogLevelFromConfiguration() error {
	logLevel, err := a.conf.GetString(logLevelConfKey)
	if err != nil {
		return err
	}

	err = log.SetLevel(logLevel)
	if err != nil {
		return errors.Wrapf(err, "Failed to set log level (%s)", logLevel).SetClass(errors.ClassBadInput)
	}

	log.WithContext(context.Background()).Infof("Set log level to: %s", logLevel)

	return nil
}

func (a *App) tracerInit() error {
	ctx := context.Background()
	tracerHost, err := a.conf.GetString(tracerHostConfKey)
	if err != nil {
		return errors.Errorf("Failed to get key (%s) from configuration", tracerHostConfKey)
	}

	tracerEnabled, err := a.conf.GetBool(tracerEnabledConfKey)
	if err != nil {
		return errors.Errorf("Failed to get key (%s) from configuration", tracerEnabledConfKey)
	}

	if tracerEnabled {
		if err := tracer.InitGlobalTracer(appName, tracerHost); err != nil {
			log.WithContext(ctx).Errorf("Could not initialize tracer %v", err)
			return err
		}
		log.WithContext(ctx).Infof("Sending traces to host: %+s", tracerHost)
	}

	return nil
}
