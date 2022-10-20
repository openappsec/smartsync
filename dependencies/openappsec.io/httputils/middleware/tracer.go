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
	"fmt"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	"openappsec.io/ctxutils"
	"openappsec.io/log"
	"openappsec.io/tracer"
)

// Tracing returns a handler function for the relevant router while tracing its execution
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var span opentracing.Span
		trace := tracer.GlobalTracer()
		opName := fmt.Sprintf("%s - %s", r.Method, r.URL.Path)

		if wireContext, err := trace.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header)); err != nil {
			span = trace.StartSpan(opName)
		} else {
			span = trace.StartSpan(opName, opentracing.ChildOf(wireContext))
		}
		defer span.Finish()

		if sc, ok := span.Context().(jaeger.SpanContext); ok {
			spanID := sc.SpanID().String()
			traceID := sc.TraceID().String()
			// add SpanID and TraceID so the Log package can extract them
			r = r.WithContext(ctxutils.Insert(r.Context(), log.EventTraceID, traceID))
			r = r.WithContext(ctxutils.Insert(r.Context(), log.EventSpanID, spanID))

			span.SetTag("span.id", spanID)
			span.SetTag("trace.id", traceID)
		}
		span.SetTag("request.id", r.Header.Get("X-Request-Id"))
		span.SetTag("correlation.id", r.Header.Get("X-Correlation-Id"))
		ext.HTTPMethod.Set(span, r.Method)
		ext.HTTPUrl.Set(span, r.URL.Path)

		// add the Span to request context so we can extract it later
		r = r.WithContext(opentracing.ContextWithSpan(r.Context(), span))
		next.ServeHTTP(w, r)
	})
}
