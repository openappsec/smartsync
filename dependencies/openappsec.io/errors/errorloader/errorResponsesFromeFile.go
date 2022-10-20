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

package errorloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"openappsec.io/ctxutils"
	"openappsec.io/errors"
)

const (
	defaultSvcCode = "199"
	// DefaultErrorResponseCode is the default error code for ErrorResponse
	DefaultErrorResponseCode = "199-000"
)

// ErrorResponse defines a REST request error response schema
type ErrorResponse struct {
	MessageID   string          `json:"messageId"`
	Message     string          `json:"message"`
	ReferenceID string          `json:"referenceId"`
	Severity    errors.Severity `json:"severity"`
}

// ErrorConfig structs holds configuration variables which can be set by the configure func
type ErrorConfig struct {
	errorsMap map[string]ErrorResponse
	svcCode   string
}

var config ErrorConfig

func init() {
	config = ErrorConfig{
		errorsMap: nil,
		svcCode:   defaultSvcCode,
	}
}

// Configure configures the errors file path & service code for the package
func Configure(errorFilePath string, svcCode string) error {
	config.svcCode = svcCode
	errorsMap, err := readErrorFromFile(errorFilePath)

	if err != nil {
		config.errorsMap = nil
		return err
	}

	config.errorsMap = errorsMap
	return nil
}

// Error implement the error interface for the ErrorResponse type
func (e *ErrorResponse) Error() string {
	body, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintf(`{"messageId": "%s","message": "%s", "referenceId": "%s", "severity": "%s"}`,
			e.MessageID, e.Message, e.ReferenceID, e.Severity)
	}
	return string(body)
}

// NewErrorResponse creates and returns a default ErrorResponse struct
func NewErrorResponse(trace string, body string) ErrorResponse {
	return ErrorResponse{
		MessageID:   config.svcCode + "-000",
		Message:     body,
		ReferenceID: trace,
		Severity:    errors.High,
	}
}

// GetError creates and returns a error response body
func GetError(ctx context.Context, errName string) (ErrorResponse, error) {
	traceID := ctxutils.ExtractString(ctx, ctxutils.ContextKeyEventTraceID)
	if config.errorsMap == nil {
		return ErrorResponse{}, errors.Errorf("No error file was loaded, use the configure func")
	}

	errorData, ok := config.errorsMap[errName]
	if !ok {
		return ErrorResponse{}, errors.Errorf("Failed to retrieve key: %s errors documentation file", errName)
	}

	errorData.MessageID = config.svcCode + "-" + errorData.MessageID
	errorData.ReferenceID = traceID

	return errorData, nil
}

// readErrorFromFile gets a file path to read from and returns the correct ErrorData
// it first looks for the errors file in working directory, and if not found - looks under the executable directory.
func readErrorFromFile(errorFilePath string) (map[string]ErrorResponse, error) {
	errorsJSON, err := ioutil.ReadFile(errorFilePath)
	if err != nil {
		exPath, err := os.Executable()
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to get executable path")
		}
		exDir := filepath.Dir(exPath)
		errorFilePath = strings.ReplaceAll(path.Join(exDir, errorFilePath), "\\", "/")
		errorsJSON, err = ioutil.ReadFile(errorFilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to read errors documentation file")
		}
	}

	var errorsMap map[string]ErrorResponse
	err = json.Unmarshal(errorsJSON, &errorsMap)
	if err != nil {
		return nil, errors.Errorf("Failed to unmarshal errors documentation file")
	}

	return errorsMap, nil
}
