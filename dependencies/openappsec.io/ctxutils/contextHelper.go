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

package ctxutils

import (
	"context"
	"time"
)

type contextKey string

// const for contextKey types
const (
	ContextKeyAgentID      = "agentId"
	ContextKeyTenantID     = "tenantId"
	ContextKeyEventTraceID = "eventTraceId"
	ContextKeyEventSpanID  = "eventSpanId"
	ContextKeyEventTags    = "eventTags"
	ContextKeySourceID     = "sourceId"
	ContextKeyProfileID    = "profileId"
	ContextKeyRequestID    = "requestId"
	ContextKeyAPIVersion   = "apiVersion"
)

// Insert gets a key and value and adds it to the context
func Insert(ctx context.Context, key string, value interface{}) context.Context {
	return context.WithValue(ctx, contextKey(key), value)
}

// Extract returns a value for a given key within the given context
func Extract(ctx context.Context, key string) interface{} {
	return ctx.Value(contextKey(key))
}

// ExtractString returns a value for a given key within the given context
func ExtractString(ctx context.Context, key string) string {
	if val, ok := ctx.Value(contextKey(key)).(string); ok {
		return val
	}
	return ""
}

// Detach returns a context that keeps all the values of its parent context
// but detaches from the cancellation and error handling.
func Detach(ctx context.Context) context.Context { return detachedContext{ctx} }

type detachedContext struct{ parent context.Context }

func (v detachedContext) Deadline() (time.Time, bool)       { return time.Time{}, false }
func (v detachedContext) Done() <-chan struct{}             { return nil }
func (v detachedContext) Err() error                        { return nil }
func (v detachedContext) Value(key interface{}) interface{} { return v.parent.Value(key) }
