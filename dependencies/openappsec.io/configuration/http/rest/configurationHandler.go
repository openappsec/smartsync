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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path"

	"openappsec.io/errors/errorloader"
	"openappsec.io/httputils/responses"
	"openappsec.io/log"
)

// Configuration exposes an interface of configuration related actions
type Configuration interface {
	SetMany(ctx context.Context, conf map[string]interface{}) error
	Get(key string) interface{}
	GetAll() map[string]interface{}
	IsSet(key string) bool
}

// AddConfigurationHandler gets a configuration service and returns an HTTP handler for 'set configuration' REST requests
func AddConfigurationHandler(conf Configuration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c map[string]interface{}
		ctx := r.Context()
		// TODO: Handle error from ioutil.ReadAll
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &c); err != nil {
			log.WithContext(ctx).Debugf("Failed to unmarshal body. error: %s", err)
			errorResponse := errorloader.NewErrorResponse("", "Invalid configuration format")
			responses.HTTPReturn(ctx, w, http.StatusBadRequest, []byte(errorResponse.Error()), true)
			return
		}

		if err := conf.SetMany(r.Context(), c); err != nil {
			log.WithContext(ctx).Debugf("Failed to add configuration. error: %s:", err)
			errorResponse := errorloader.NewErrorResponse("", "Invalid configuration format")
			responses.HTTPReturn(ctx, w, http.StatusBadRequest, []byte(errorResponse.Error()), true)
			return
		}

		responses.HTTPReturn(ctx, w, http.StatusOK, nil, true)
	})
}

// RetrieveEntireConfigurationHandler gets a configuration service and returns an HTTP handler for'set configuration' REST requests
func RetrieveEntireConfigurationHandler(conf Configuration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		conf, err := json.Marshal(conf.GetAll())
		if err != nil {
			log.WithContext(ctx).Errorln("Failed to marshal:", err)
			errorResponse := errorloader.NewErrorResponse("", err.Error())
			responses.HTTPReturn(ctx, w, http.StatusInternalServerError, []byte(errorResponse.Error()), true)
			return
		}

		responses.HTTPReturn(ctx, w, http.StatusOK, conf, true)
	})
}

// RetrieveConfigurationHandler gets a configuration service and returns an HTTP handler for 'set configuration' REST requests
func RetrieveConfigurationHandler(conf Configuration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		key := path.Base(r.URL.Path)
		if !conf.IsSet(key) {
			log.WithContext(ctx).Debug("configuration key is not set")
			errorResponse := errorloader.NewErrorResponse("", "Requested configuration key is not set")
			responses.HTTPReturn(ctx, w, http.StatusNotFound, []byte(errorResponse.Error()), true)
			return
		}

		conf, err := json.Marshal(conf.Get(key))
		if err != nil {
			log.WithContext(ctx).Errorln("Failed to marshal:", err)
			errorResponse := errorloader.NewErrorResponse("", "Requested configuration key is not set")
			responses.HTTPReturn(ctx, w, http.StatusInternalServerError, []byte(errorResponse.Error()), true)
			return
		}

		responses.HTTPReturn(ctx, w, http.StatusOK, conf, true)
	})
}
