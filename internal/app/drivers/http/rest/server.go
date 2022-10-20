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

package rest

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"openappsec.io/smartsync-service/models"
	"openappsec.io/errors"
	"openappsec.io/errors/errorloader"
	"openappsec.io/health"
	"openappsec.io/log"
)

const (
	serverConfBaseKey    = "server"
	serverPortConfKey    = serverConfBaseKey + ".port"
	serverTimeoutConfKey = serverConfBaseKey + ".timeout"

	errorsConfBaseKey = "errors"
	errorsFilePathKey = errorsConfBaseKey + ".filepath"
	errorsCodeKey     = errorsConfBaseKey + ".code"
)

// Configuration exposes an interface of configuration related actions
type Configuration interface {
	SetMany(ctx context.Context, conf map[string]interface{}) error
	Set(ctx context.Context, key string, value interface{}) error
	Get(key string) interface{}
	GetString(key string) (string, error)
	GetInt(key string) (int, error)
	GetDuration(key string) (time.Duration, error)
	GetAll() map[string]interface{}
	IsSet(key string) bool
}

// HealthService exposes an interface of health related actions
type HealthService interface {
	Live() health.LivenessResponse
	Ready(ctx context.Context) health.ReadinessResponse
	AddReadinessChecker(checker health.Checker)
}

// AppService exposes the domain interface for handling events
type AppService interface {
	ProcessSyncRequest(ctx context.Context, ids models.SyncID) error
	ReadS3Files(ctx context.Context, ids models.SyncID) ([]byte, error)
}

// Server http server interface
type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// Adapter server adapter
type Adapter struct {
	server Server

	appSrv AppService

	conf Configuration

	healthSvc HealthService
}

// Start REST webserver
func (a *Adapter) Start() error {
	log.WithContext(context.Background()).Infoln("Server is starting...")
	return a.server.ListenAndServe()
}

// Stop REST webserver
func (a *Adapter) Stop(ctx context.Context) error {
	var stopError error

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	log.WithContext(ctx).Infoln("Shutting down server...")
	if err := a.server.Shutdown(ctx); err != nil {
		stopError = errors.New("Failed to gracefully stop server")
	}
	return stopError
}

// NewAdapter is a rest adapter provider
func NewAdapter(cs Configuration, hs HealthService, appsvc AppService) (*Adapter, error) {
	ra := Adapter{
		appSrv:    appsvc,
		conf:      cs,
		healthSvc: hs,
	}

	serverTimeout, err := cs.GetDuration(serverTimeoutConfKey)
	if err != nil {
		return nil, err
	}

	r := ra.newRouter(serverTimeout)
	server := &http.Server{
		Handler: r,
	}

	errorsPath, err := cs.GetString(errorsFilePathKey)
	if err != nil {
		return nil, err
	}

	errorsCode, err := cs.GetString(errorsCodeKey)
	if err != nil {
		return nil, err
	}

	// init the error loader singleton
	err = errorloader.Configure(errorsPath, errorsCode)
	if err != nil {
		return nil, err
	}

	port, err := cs.GetInt(serverPortConfKey)
	if err != nil {
		return nil, err
	}

	if port > 0 {
		server.Addr = ":" + strconv.Itoa(port)
	}

	ra.server = server

	return &ra, nil
}
