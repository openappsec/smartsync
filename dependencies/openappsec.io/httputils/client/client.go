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

package client

import (
	"fmt"
	"net/http"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"openappsec.io/errors"
	"openappsec.io/tracer"
)

// tracerTransport implements the http.RoundTripper interface
type tracerTransport struct {
	http.RoundTripper
}

// RoundTrip starts a span from the request context and inject it to the request headers
func (t tracerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.RoundTripper == nil {
		t.RoundTripper = http.DefaultTransport
	}

	tc := tracer.GlobalTracer()
	opName := fmt.Sprintf("%s - %s", req.Method, req.URL.String())
	span, ctx := opentracing.StartSpanFromContextWithTracer(req.Context(), tc, opName)
	defer span.Finish()
	// Inject the span context into the request headers
	if err := tc.Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header)); err != nil {
		return nil, errors.Wrap(err, "Failed to inject tracer headers")
	}
	req = req.WithContext(ctx)

	ext.SpanKindRPCClient.Set(span)
	ext.HTTPUrl.Set(span, req.URL.String())
	ext.HTTPMethod.Set(span, req.Method)

	return t.RoundTripper.RoundTrip(req)
}

// NewTracerClient returns new http client with tracerTransport
func NewTracerClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &tracerTransport{},
		Timeout:   timeout,
	}
}
