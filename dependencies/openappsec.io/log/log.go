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

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"

	"openappsec.io/ctxutils"
	"openappsec.io/errors"

	"github.com/sirupsen/logrus"
)

const (
	// PanicLevel level, highest level of severity. Logs and then calls panic with the
	// message passed to Debug, Info, ...
	PanicLevel Level = iota
	// FatalLevel level. Logs and then calls `logger.Exit(1)`. It will exit even if the
	// logging level is set to Panic.
	FatalLevel
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// Commonly used for hooks to send errors to an error tracking service.
	ErrorLevel
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel
	// InfoLevel level. General operational entries about what's going on inside the
	// application.
	InfoLevel
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	TraceLevel
)

// amount of frames to skip in order to get caller function/line/file
// this is constant as only the first operation on a log should add this data
const skip = 3

// Fields type, used to pass to `WithFields`.
type Fields logrus.Fields

// Level is logrus.Level
type Level logrus.Level

// Log is a log instance
type Log struct {
	entry *logrus.Entry
}

var outputer *logrus.Logger

var baseContextFields = []string{
	ctxutils.ContextKeyTenantID,
	ctxutils.ContextKeyEventTraceID,
	ctxutils.ContextKeyEventTags,
	ctxutils.ContextKeyEventSpanID,
}

// RFC3339MillisFormat is a best practice RFC3339 time format with milliseconds
const RFC3339MillisFormat = "2006-01-02T15:04:05.000Z07:00"

var baseValues = Format{
	EventTime:         "",
	EventAudience:     "Internal",
	EventAudienceTeam: "",
	EventLogLevel:     "",
	EventName:         "log",
	EventType:         "Code Related",
	EventSource:       EventSource{},
	EventTags:         []string{},
	EventData: EventData{
		MessageData: map[string]interface{}{},
	},
	EventPriority: EventPriorityMedium,
	EventSeverity: EventSeverityInfo,
}

func init() {
	outputer = logrus.New()
	outputer.SetFormatter(newFormatter())
	outputer.SetLevel(logrus.InfoLevel)
}

func newLog() *Log {
	l := Log{
		entry: logrus.NewEntry(outputer),
	}
	return l.logWithSourceInfo(skip)
}

// AddHook adds a logrus hook which called on every log
func AddHook(hook logrus.Hook) {
	outputer.AddHook(hook)
}

// SetOutput sets the output for this Logger to the given io.writer
func SetOutput(out io.Writer) {
	outputer.SetOutput(out)
}

// SetStaticLogField sets the default value of a given log field to the given value
// This default value will be outputted with every log unless it is overridden
func SetStaticLogField(fieldKey string, fieldValue interface{}) error {
	if err := baseValues.intoFormat(fieldKey, fieldValue, true); err != nil {
		return errors.Wrapf(err, "Invalid base field specified")
	}
	return nil
}

// AddContextField adds a field which will be searched for within the given context by the Logger
// these fields, if found within the given context, are added to the outputted log
func AddContextField(field string) {
	baseContextFields = append(baseContextFields, field)
}

// RemoveContextField removes a field to be searched for by the logger within a given context
func RemoveContextField(field string) {
	temp := make([]string, 0, len(baseContextFields)-1)
	for _, f := range baseContextFields {
		if f != field {
			temp = append(temp, f)
		}
	}
	baseContextFields = temp
}

// GetLevel returns log level
func GetLevel() Level {
	return Level(outputer.GetLevel())
}

// SetLevel sets log level
func SetLevel(level string) error {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}

	outputer.SetLevel(lvl)
	return nil
}

// WithContext instantiates a log and calls log.WithContext on it
func WithContext(ctx context.Context) *Log {
	return newLog().WithContext(ctx)
}

// WithContextAndEventID instantiates a log with event ID and calls log.WithContext on it
func WithContextAndEventID(ctx context.Context, eventID string) *Log {
	l := newLog().WithContext(ctx)
	l.entry = l.entry.WithFields(logrus.Fields{EventID: eventID})
	return l
}

// WithContext extracts context fields to create an entry
func (l *Log) WithContext(ctx context.Context) *Log {
	if baseContextFields != nil {
		for _, k := range baseContextFields {
			v := ctxutils.Extract(ctx, k)
			if v != nil {
				l.entry = l.entry.WithFields(logrus.Fields{k: v})
			}
		}
	}

	return l
}

// WithFields instantiates a log and calls log.WithFields on it
func WithFields(fields Fields) *Log {
	return newLog().WithFields(fields)
}

// WithFields adds field to the log. All fields would be flattened to a string representation except fields under KnownStringSliceFields
func (l *Log) WithFields(fields Fields) *Log {
	flatFields := make(Fields, len(fields))
	for key, val := range fields {
		if val != nil {
			// if the "key" is in KnownStringSliceFields, keep its value as is (i.e, keep it as a slice)
			_, isKnownField := KnownStringSliceFields[key]
			if isKnownField {
				flatFields[key] = val
			} else {
				switch val.(type) {
				case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
					// For numeric value, keep the value as it's
					flatFields[key] = val
				default:
					// Otherwise, turn the value into it's string representation
					flatFields[key] = fmt.Sprintf("%+v", val)
				}
			}
		}
	}
	l.entry = l.entry.WithFields(logrus.Fields(flatFields))

	return l
}

// WithEventID add event ID value to log
func (l *Log) WithEventID(eventID string) *Log {
	l.entry = l.entry.WithField(EventID, eventID)
	return l
}

// WithContextAndFields instantiates a log and calls log.WithContext.WithFields on it
// this is for backwards compatibility
func WithContextAndFields(ctx context.Context, fields Fields) *Log {
	return newLog().WithContext(ctx).WithFields(fields)
}

func (f *Format) handleKnownStringField(k string, vString string) {
	switch k {
	// general fields
	case EventName:
		f.EventName = vString
	case EventType:
		f.EventType = vString
	case EventAudience:
		f.EventAudience = vString
	case EventAudienceTeam:
		f.EventAudienceTeam = vString
	case EventFrequency:
		f.EventFrequency = vString
	case EventSeverity:
		f.EventSeverity = vString
	case EventPriority:
		f.EventPriority = vString

	// source fields
	case FogType:
		f.EventSource.FogType = vString
	case FogID:
		f.EventSource.FogID = vString
	case EventTraceID:
		f.EventSource.EventTraceID = vString
	case TenantID:
		f.EventSource.TenantID = vString
	case AgentID:
		f.EventSource.AgentID = vString
	case EventSpanID:
		f.EventSource.EventSpanID = vString
	case CodeLabel:
		f.EventSource.CodeLabel = vString
	case IssuingFunction:
		f.EventSource.IssuingFunction = vString
	case IssuingFile:
		f.EventSource.IssuingFile = vString
	case IssuingLine:
		f.EventSource.IssuingLine = vString
	case EventID:
		f.EventSource.EventID = vString

	// message data fields
	case StateKey:
		f.EventData.StateKey = vString
	}
}

func (f *Format) handleKnownStringSliceField(k string, vSlice []string) {
	switch k {
	case EventTags:
		f.EventTags = vSlice
	}
}

func (f *Format) intoFormat(k string, v interface{}, strict bool) error {

	if _, ok := KnownStringFields[k]; ok {
		if vString, ok := v.(string); ok {
			f.handleKnownStringField(k, vString)
		} else if strict { // if the field is a known string field but the val is not a string
			return errors.Errorf("Invalid value type specified for log field (%s), expected string", k)
		}
		return nil
	}

	if _, ok := KnownStringSliceFields[k]; ok {
		if vSlice, ok := v.([]string); ok {
			f.handleKnownStringSliceField(k, vSlice)
		} else if strict {
			return errors.Errorf("Invalid value type specified for log field (%s), expected string slice", k)
		}
		return nil
	}

	f.EventData.MessageData[k] = v
	return nil
}

// Variable used for entryWithSourceInfo global caching
var (
	codeInfoMap         sync.Map
	unknownCodeLocation = Fields{
		IssuingFile:     "unknown",
		IssuingLine:     "-1",
		IssuingFunction: "unknown",
	}
)

func (l *Log) logWithSourceInfo(skip int) *Log {
	p, file, line, okCaller := runtime.Caller(skip)
	if !okCaller {
		return l.WithFields(unknownCodeLocation)
	}

	if mapCodeInfo, ok := codeInfoMap.Load(p); ok {
		return l.WithFields(mapCodeInfo.(Fields))
	}

	funcName := "unknown"
	fn := runtime.FuncForPC(p)
	if fn != nil {
		funcName = fn.Name()
	}
	codeInfo := Fields{
		IssuingFile:     file,
		IssuingLine:     fmt.Sprintf("%v", line),
		IssuingFunction: funcName,
	}
	codeInfoMap.Store(p, codeInfo)

	return l.WithFields(codeInfo)
}
