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

package tracer

import (
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-client-go/zipkin"
	"openappsec.io/errors"
)

var tracer opentracing.Tracer

// init initialize new tracer with NoopTracer
func init() {
	tracer = opentracing.NoopTracer{}
	opentracing.SetGlobalTracer(tracer)
}

// InitGlobalTracer initialize new tracer with given name and address
func InitGlobalTracer(svcName, tracerAddress string) error {
	cfg := config.Configuration{
		ServiceName: svcName,
		Sampler: &config.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LocalAgentHostPort: tracerAddress,
		},
	}
	zipkinPropagator := zipkin.NewZipkinB3HTTPHeaderPropagator()
	injector := config.Injector(opentracing.HTTPHeaders, zipkinPropagator)
	extractor := config.Extractor(opentracing.HTTPHeaders, zipkinPropagator)
	var err error
	tracer, _, err = cfg.NewTracer(injector, extractor)
	if err != nil {
		return errors.Errorf("failed to initialize tracer", err)
	}

	opentracing.SetGlobalTracer(tracer)
	return nil
}

// GlobalTracer returns the global tracer
func GlobalTracer() opentracing.Tracer {
	return tracer
}
