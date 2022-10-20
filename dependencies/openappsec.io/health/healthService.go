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

package health

import (
	"context"
	"fmt"
	"time"
)

// Service implements the health service interface
type Service struct {
	bootTime        time.Time
	readinessChecks []Checker
}

// LivenessResponse defines the Health service response to a liveness query
type LivenessResponse struct {
	Up        bool      `json:"up"`
	Timestamp time.Time `json:"timestamp"`
}

// ReadinessResponse defines the Health service response to a readiness query
type ReadinessResponse struct {
	Ready        bool              `json:"ready"`
	Uptime       string            `json:"uptime"`
	CheckResults map[string]string `json:"checkResults"`
	Timestamp    time.Time         `json:"timestamp"`
}

// Checker is an interface which defines a backend check that returns the check name and an error if it fails
type Checker interface {
	HealthCheck(ctx context.Context) (string, error)
}

// NewService provides a new health service
func NewService() *Service {
	return &Service{
		bootTime: time.Now(),
	}
}

// AddReadinessChecker adds a Check for readiness
func (hs *Service) AddReadinessChecker(checker Checker) {
	hs.readinessChecks = append(hs.readinessChecks, checker)
}

// Live is a stub implementation for liveness checks
func (hs *Service) Live() LivenessResponse {
	return LivenessResponse{
		Up:        true,
		Timestamp: time.Now(),
	}
}

// Ready is a stub implementation for readiness checks
func (hs *Service) Ready(ctx context.Context) ReadinessResponse {
	results, ok := collectChecks(ctx, hs.readinessChecks)
	return ReadinessResponse{
		Timestamp:    time.Now(),
		Ready:        ok,
		CheckResults: results,
		Uptime:       fmt.Sprintf("%s", time.Since(hs.bootTime)),
	}
}

func collectChecks(ctx context.Context, checkers []Checker) (map[string]string, bool) {
	// create a channel for passing check result
	type checkResult struct {
		checkName string
		err       error
	}
	cCheckResult := make(chan checkResult)
	defer close(cCheckResult)

	// run checkers concurrently
	for _, checker := range checkers {
		go func(checker Checker) {
			checkName, err := checker.HealthCheck(ctx)
			cCheckResult <- checkResult{
				checkName: checkName,
				err:       err,
			}
		}(checker)
	}

	// collect results
	allChecksUp := true
	results := make(map[string]string)
	for i := 0; i < len(checkers); i++ {
		select {
		case checkRes := <-cCheckResult:
			if checkRes.err != nil {
				allChecksUp = false
				results[checkRes.checkName] = checkRes.err.Error()
			} else {
				results[checkRes.checkName] = "OK"
			}
		}
	}

	return results, allChecksUp
}
