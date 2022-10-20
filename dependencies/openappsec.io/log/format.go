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

package log

// consts for fields in the log, allowing users to set values for these fields
const (
	// general fields
	EventName         = "eventName"
	EventType         = "eventType"
	EventAudience     = "eventAudience"
	EventAudienceTeam = "eventAudienceTeam"
	EventFrequency    = "eventFrequency"
	EventSeverity     = "eventSeverity"
	EventPriority     = "eventPriority"

	// source fields
	FogID           = "fogId"
	FogType         = "fogType"
	EventTraceID    = "eventTraceId"
	TenantID        = "tenantId"
	AgentID         = "agentId"
	EventSpanID     = "eventSpanId"
	CodeLabel       = "codeLabel"
	IssuingFunction = "issuingFunction"
	IssuingFile     = "issuingFile"
	IssuingLine     = "issuingLine"
	EventID         = "eventId"

	// message data fields
	StateKey = "stateKey"

	// labels
	EventTags = "eventTags"

	// event severity
	// EventSeverityInfo is the default value of event severity
	EventSeverityInfo     = "Info"
	EventSeverityCritical = "Critical"
	EventSeverityHigh     = "High"

	// event priority
	// EventPriorityMedium is the default value of event priority
	EventPriorityMedium = "Medium"
)

// KnownStringFields are all the string type fields that a user can configure in a log
// via the "WithContext" or "WithContextAndFields" functions
var KnownStringFields = map[string]string{
	EventName:         "",
	EventType:         "",
	EventAudience:     "",
	EventAudienceTeam: "",
	EventFrequency:    "",
	FogID:             "",
	FogType:           "",
	EventTraceID:      "",
	TenantID:          "",
	AgentID:           "",
	EventSpanID:       "",
	CodeLabel:         "",
	IssuingFunction:   "",
	IssuingFile:       "",
	IssuingLine:       "",
	StateKey:          "",
	EventSeverity:     "",
	EventPriority:     "",
	EventID:           "",
}

// KnownStringSliceFields are all the string slice type fields that a user can configure in a log
// via the "WithContext" or "WithContextAndFields" functions
var KnownStringSliceFields = map[string]string{
	EventTags: "",
}

// Format is the logging format
type Format struct {
	EventTime         string      `json:"eventTime"`
	EventName         string      `json:"eventName"`
	EventType         string      `json:"eventType"`
	EventAudience     string      `json:"eventAudience"`
	EventAudienceTeam string      `json:"eventAudienceTeam"`
	EventFrequency    string      `json:"eventFrequency"`
	EventLogLevel     string      `json:"eventLogLevel"`
	EventSource       EventSource `json:"eventSource"`
	EventData         EventData   `json:"eventData"`
	EventTags         []string    `json:"eventTags"`
	EventPriority     string      `json:"eventPriority"`
	EventSeverity     string      `json:"eventSeverity"`
}

// EventSource is the event source data
type EventSource struct {
	FogID           string `json:"fogId"`
	FogType         string `json:"fogType"`
	EventTraceID    string `json:"eventTraceId"`
	TenantID        string `json:"tenantId"`
	AgentID         string `json:"agentId"`
	EventSpanID     string `json:"eventSpanId"`
	CodeLabel       string `json:"codeLabel"`
	IssuingFunction string `json:"issuingFunction"`
	IssuingFile     string `json:"issuingFile"`
	IssuingLine     string `json:"issuingLine"`
	EventID         string `json:"eventId"`
}

// EventData is the event data data
type EventData struct {
	StateKey     string                 `json:"stateKey"`
	EventMessage string                 `json:"eventMessage"`
	MessageData  map[string]interface{} `json:"messageData"`
}
