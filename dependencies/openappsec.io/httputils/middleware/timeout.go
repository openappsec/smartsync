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

package middleware

import (
	"net/http"
	"time"
)

// Timeout returns a timeout handler function for the relevant router.
// When reaching the given `timeout`, the handler cancels the ctx and returns a 503 Service Unavailable status
// with the given `errMsg` body to the client.
//
// Note: The request execution doesn't stop when the context is cancelled.
// The context status should be checked when performing state-change operations (such as db operations, http requests etc.) and heavy (cancelable) calculations.
// In common libraries (such as mongo, http, etc.) there is usually already a ctx status check - no code additions needed.
func Timeout(timeout time.Duration, errMsg string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, errMsg)
	}
}
