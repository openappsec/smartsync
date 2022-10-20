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
	"encoding/json"
	"fmt"
	"math"

	"github.com/sirupsen/logrus"
)

const (
	// FieldNameRequestBody is the name of the field used to keep the request body under
	FieldNameRequestBody = "requestBody"

	// FieldNameReqBodyPartNum is the name of the field presenting which part out of how many parts
	// is presented under the requestBody field
	FieldNameReqBodyPartNum = "requestBodyPartNumber"

	// This is an estimated max size (in bytes) of the log field requestBody.
	//It is calculated as 80% of 16kb (the allowed max size of the entire log entry)
	maxSizeLogFieldRequestBody = 0.8 * 16 * 1024 //TODO (maybe 0.7 ??) --> 12/16
)

// createPartialReqBodyData is a helper function used when splitting the field requestbody.
// It keeps all the fields of the original entry, except two:
// 1. The requestBody field would contain the partial data
// 2. New field named requestBodyPartNumber to indicate which part is it and out of how many parts
func createPartialReqBodyData(partNum, totalParts int, partialReqBody string, entry *logrus.Entry) logrus.Fields {
	res := logrus.Fields{}
	// Copy all fields except requestBody field.
	for k, v := range entry.Data {

		if k == FieldNameRequestBody {
			continue
		}
		res[k] = v
	}
	// Add the partial requestBody & the part number to the fields
	res[FieldNameReqBodyPartNum] = fmt.Sprintf("%d/%d", partNum, totalParts)
	res[FieldNameRequestBody] = partialReqBody
	return res
}

func splitEntry(entry *logrus.Entry) []*logrus.Entry {
	// Step 1: Check if requestBody is one of the fields in entry.Data - if not, return
	val, ok := entry.Data[FieldNameRequestBody]
	if !ok {
		return []*logrus.Entry{entry}
	}

	//Step 2: Assert the requestBody is indeed string - if not, make it one
	requestBody, ok := val.(string)
	if !ok {
		requestBody = fmt.Sprintf("%+v", val)
	}

	// Step 3: Split the requestBody into chunks of maxSize length
	fMaxSize := maxSizeLogFieldRequestBody
	maxSize := int(fMaxSize)
	var splitReqBody []string
	if len(requestBody) > maxSize {
		for i := 0; i < len(requestBody); i += maxSize {
			stop := int(math.Min(float64(i+maxSize), float64(len(requestBody))))
			splitReqBody = append(splitReqBody, requestBody[i:stop])
		}
	} else {
		// No need to split - just return
		return []*logrus.Entry{entry}
	}

	// Step 4: For each chunk, create a different log entry, identical to the original except for the Data part
	var res []*logrus.Entry
	for i, reqBodyPart := range splitReqBody {
		e := &logrus.Entry{
			Logger:  entry.Logger,
			Data:    createPartialReqBodyData(i+1, len(splitReqBody), reqBodyPart, entry),
			Time:    entry.Time,
			Level:   entry.Level,
			Caller:  entry.Caller,
			Message: entry.Message,
			Buffer:  entry.Buffer,
			Context: entry.Context,
		}
		res = append(res, e)
	}
	return res
}

// Debugf creates a formatted log with debug level
func Debugf(format string, args ...interface{}) { newLog().Debugf(format, args...) }

// Debugf creates a formatted log with debug level
func (l *Log) Debugf(format string, args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Debugf(format, args...)
	}
}

// Infof creates a formatted log with info level
func Infof(format string, args ...interface{}) { newLog().Infof(format, args...) }

// Infof creates a formatted log with info level
func (l *Log) Infof(format string, args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Infof(format, args...)
	}
}

// Warnf creates a formatted log with warning level
func Warnf(format string, args ...interface{}) { newLog().Warnf(format, args...) }

// Warnf creates a formatted log with warning level
func (l *Log) Warnf(format string, args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Warnf(format, args...)
	}
}

// Errorf creates a formatted log with error level
func Errorf(format string, args ...interface{}) { newLog().Errorf(format, args...) }

// Errorf creates a formatted log with error level
func (l *Log) Errorf(format string, args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Errorf(format, args...)
	}
}

// Debugln creates a log with debug level
func Debugln(args ...interface{}) { newLog().Debugln(args...) }

// Debugln creates a log with debug level
func (l *Log) Debugln(args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Debugln(args...)
	}
}

// Infoln creates a log with info level
func Infoln(args ...interface{}) { newLog().Infoln(args...) }

// Infoln creates a log with info level
func (l *Log) Infoln(args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Infoln(args...)
	}
}

// Warnln creates a log with warning level
func Warnln(args ...interface{}) { newLog().Warnln(args...) }

// Warnln creates a log with warning level
func (l *Log) Warnln(args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Warnln(args...)
	}
}

// Errorln creates a log with error level
func Errorln(args ...interface{}) { newLog().Errorln(args...) }

// Errorln creates a log with error level
func (l *Log) Errorln(args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Errorln(args...)
	}
}

// Debug creates a log with debug level
func Debug(args ...interface{}) { newLog().Debug(args...) }

// Debug creates a log with debug level
func (l *Log) Debug(args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Debug(args...)
	}
}

// Info creates a log with info level
func Info(args ...interface{}) { newLog().Info(args...) }

// Info creates a log with info level
func (l *Log) Info(args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Info(args...)
	}
}

// Warn creates a log with warning level
func Warn(args ...interface{}) { newLog().Warn(args...) }

// Warn creates a log with warning level
func (l *Log) Warn(args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Warn(args...)
	}
}

// Error creates a log with error level
func Error(args ...interface{}) { newLog().Error(args...) }

// Error creates a log with error level
func (l *Log) Error(args ...interface{}) {
	entries := splitEntry(l.entry)
	for _, e := range entries {
		e.Error(args...)
	}
}

type formatter struct{}

// Format takes a logrus log entry and converts it to the desired log format, placing each field in its correct location within the log struct
func (u formatter) Format(e *logrus.Entry) ([]byte, error) {

	l := baseValues                                    // copy static values
	l.EventData.MessageData = map[string]interface{}{} // create new map to avoid modifying baseValues
	for k, v := range baseValues.EventData.MessageData {
		l.EventData.MessageData[k] = v
	}

	l.EventLogLevel = e.Level.String()
	if e.Level == logrus.WarnLevel {
		l.EventSeverity = EventSeverityHigh
	} else if e.Level == logrus.ErrorLevel {
		l.EventSeverity = EventSeverityCritical
	}

	for k, v := range e.Data {
		_ = l.intoFormat(k, v, false) // ok to ignore error since false means non-strict mode
	}

	l.EventData.EventMessage = e.Message

	l.EventTime = e.Time.UTC().Format(RFC3339MillisFormat)

	res, err := json.Marshal(l)
	if err == nil {
		res = append(res, '\n')
	}
	return res, err
}

// newFormatter creates a custom log formatter
func newFormatter() logrus.Formatter {
	return formatter{}
}
