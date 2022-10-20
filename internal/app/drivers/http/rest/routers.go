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
	"time"

	confhandlers "openappsec.io/configuration/http/rest"
	"openappsec.io/errors/errorloader"
	healthhandlers "openappsec.io/health/http/rest"
	"openappsec.io/httputils/middleware"
	"openappsec.io/log"

	"github.com/go-chi/chi"
	chimiddleware "github.com/go-chi/chi/middleware"
)

const (
	headerKeyCorrelationID = "X-Correlation-Id"
	headerKeyTraceID       = "X-Trace-Id"
	headerKeyRequestID     = "X-Request-Id"
)

// newRouter returns a router including method, path, name, and handler
func (a *Adapter) newRouter(timeout time.Duration) *chi.Mux {
	router := chi.NewRouter()
	router.Use(chimiddleware.Timeout(timeout))

	errInternal := http.StatusText(http.StatusInternalServerError)

	router.Group(
		func(router chi.Router) {
			router.Route(
				"/health", func(r chi.Router) {
					r.Get("/live", healthhandlers.LivenessHandler(a.healthSvc).ServeHTTP)
					r.Get("/ready", healthhandlers.ReadinessHandler(a.healthSvc).ServeHTTP)
				},
			)

			router.Route(
				"/configuration", func(r chi.Router) {
					r.Get("/", confhandlers.RetrieveEntireConfigurationHandler(a.conf).ServeHTTP)
					r.Post("/", confhandlers.AddConfigurationHandler(a.conf).ServeHTTP)
					r.Get("/{key}", confhandlers.RetrieveConfigurationHandler(a.conf).ServeHTTP)
				},
			)
		},
	)

	router.Group(
		func(router chi.Router) {
			router.Use(middleware.CorrelationID("missing correlation ID"))
			router.Use(middleware.TenantID("missing tenant id"))
			router.Use(middleware.Tracing)
			router.Use(
				middleware.LogWithHeader(
					errInternal,
					[]string{
						headerKeyCorrelationID,
						headerKeyRequestID,
						headerKeyTraceID,
					},
				),
			)

			router.Route(
				"/api", func(r chi.Router) {
					r.Post("/sync", a.NotifySync)
				},
			)
		},
	)

	router.Group(func(router chi.Router) {
		router.Use(middleware.CorrelationID("missing correlation ID"))
		router.Use(middleware.TenantID("missing tenant id"))
		router.Use(middleware.Tracing)
		router.Use(
			middleware.LogWithHeader(
				errInternal,
				[]string{
					headerKeyCorrelationID,
					headerKeyRequestID,
					headerKeyTraceID,
				},
			),
		)

		router.Route("/learning", func(r chi.Router) {
			r.Get("/asset_data", a.RetrieveAssetLearningData)
		})
	},
	)

	return router
}

func createErrorBody(errorName string) string {
	errorResponse, err := errorloader.GetError(context.Background(), errorName)
	if err != nil {
		log.Errorf(err.Error())
		errorBody := errorloader.NewErrorResponse("", http.StatusText(http.StatusInternalServerError))
		return (&errorBody).Error()
	}
	return errorResponse.Error()
}
