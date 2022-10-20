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

package responses

import (
	"context"
	"net/http"

	"openappsec.io/ctxutils"
	"openappsec.io/errors/errorloader"
	"openappsec.io/log"
)

// PaginationBase defines a base structure for all paginated responses
type PaginationBase struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

func debugLogResponse(ctx context.Context, code int, body []byte) {
	if log.GetLevel() >= log.DebugLevel {
		log.WithContextAndFields(ctx,
			log.Fields{
				"status":       code,
				"responseBody": string(body),
			}).Debugln("finished handling request")
	}
}

func setDefaultHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
}

func httpWriteError(ctx context.Context, w http.ResponseWriter) {
	log.WithContext(ctx).Errorln("Failed to write HTTP response")
	traceID := ctxutils.ExtractString(ctx, ctxutils.ContextKeyEventTraceID)
	errorResponse := errorloader.NewErrorResponse(traceID, http.StatusText(http.StatusInternalServerError))
	errorResponseStr := (&errorResponse).Error()
	http.Error(w, errorResponseStr, http.StatusInternalServerError)
	debugLogResponse(ctx, http.StatusInternalServerError, []byte(errorResponseStr))
}

// HTTPReturn returns response with given body and status code
func HTTPReturn(ctx context.Context, w http.ResponseWriter, code int, body []byte, wantLog bool) {
	setDefaultHeaders(w)
	w.WriteHeader(code)

	if _, err := w.Write(body); err != nil {
		if err == http.ErrHandlerTimeout {
			log.WithContextAndFields(ctx,
				log.Fields{
					"status": http.StatusServiceUnavailable,
				}).Warnln("finished handling request - context reached timeout")
			return
		}
		httpWriteError(ctx, w)
		return
	}

	if wantLog {
		debugLogResponse(ctx, code, body)
	}
}
