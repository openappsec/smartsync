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

package models

import (
	"context"
	"time"
)

// mockgen -destination mocks/mock_syncHandler.go -package mocks openappsec.io/smartsync-service/models SyncHandler

// State defines the operations on a learning state
type State interface {
	ShouldRebase() bool
	GetFilePath(id SyncID) string
	GetOriginalPath(ids SyncID) string
}

//SyncHandler defines a handler for sync per type
type SyncHandler interface {
	NewDataStruct() interface{}
	MergeData(data interface{})
	NewState() State
	ProcessData(ctx context.Context, state State) State
	GetDependencies() map[SyncID]SyncHandler
	SetCompressionEnabled()
}

//EventTypeEnum enum for event types
type EventTypeEnum string

//events type enums
const (
	EventTypeParam  = "parameterName"
	EventTypeSource = "source"
)

//TuneEvent defines a tuning decision
type TuneEvent struct {
	Decision   string        `json:"decision"`
	EventType  EventTypeEnum `json:"eventType"`
	EventTitle string        `json:"eventTitle"`
}

//TuningEvents defines multiple decisions
type TuningEvents struct {
	Decisions []TuneEvent `json:"decisions"`
}

//ConfidenceParams defines the parameters for the confidence calculator
type ConfidenceParams struct {
	MinSources     int
	MinIntervals   int
	RatioThreshold float64
	NullObject     string
	Interval       time.Duration
}
