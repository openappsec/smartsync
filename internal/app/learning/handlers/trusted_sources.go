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

package handlers

import (
	"context"
	"fmt"

	"openappsec.io/log"
	"openappsec.io/smartsync-service/models"
)

// TrustedSourcesHandler represents a trusted sources handler
type TrustedSourcesHandler struct {
	id         models.SyncID
	trustedLog mms
}

type trustedSourcesState struct {
	Logger []mapToMapToSet `json:"logger"`
}

// GetFilePath returns the path to the state file saved by the service
func (ts *trustedSourcesState) GetFilePath(id models.SyncID) string {
	return fmt.Sprintf("/%v/%v/%v/remote/data.data", id.TenantID, id.AssetID, id.Type)
}

// GetOriginalPath returns the path to the state file saved by the agent
func (ts *trustedSourcesState) GetOriginalPath(id models.SyncID) string {
	return fmt.Sprintf("/%v/%v/%v/processed/data.data", id.TenantID, id.AssetID, id.Type)
}

func (ts *trustedSourcesState) ShouldRebase() bool {
	return false
}

// NewTrustedSources creates new trusted sources handler
func NewTrustedSources(id models.SyncID) *TrustedSourcesHandler {
	return &TrustedSourcesHandler{id: id, trustedLog: mms{}}
}

// NewDataStruct returns a struct representation of the data put by agent
func (ts *TrustedSourcesHandler) NewDataStruct() interface{} {
	return &trustedSourcesState{[]mapToMapToSet{}}
}

// MergeData merges the data from agent into struct member
func (ts *TrustedSourcesHandler) MergeData(data interface{}) {
	state := data.(*trustedSourcesState)
	mergeAndConvertCerealToGo(state.Logger, ts.trustedLog, false)
}

// NewState returns a struct representing the state
func (ts *TrustedSourcesHandler) NewState() models.State {
	return &trustedSourcesState{[]mapToMapToSet{}}
}

// SetCompressionEnabled set a flag that compression is supported
func (ts *TrustedSourcesHandler) SetCompressionEnabled() {
	// do nothing
}

// ClearMergedData releases references to merged trusted sources data to free memory between runs.
func (ts *TrustedSourcesHandler) ClearMergedData() {
	ts.trustedLog = mms{}
}

func containsTrustedSourcePtrs(sources []*string, trustedMap map[string]bool) bool {
	// Check if any source is trusted
	for _, srcPtr := range sources {
		if _, ok := trustedMap[*srcPtr]; ok {
			return true
		}
	}
	return false
}

// ProcessDataFromCentralData processes data from central data
func (ts *TrustedSourcesHandler) ProcessDataFromCentralData(ctx context.Context, state models.State, mergedData *models.CentralData) models.State {
	log.WithContext(ctx).Debugf("TrustedSourcesHandler.ProcessDataFromCentralData ids: %+v", ts.id)
	ts.trustedLog = mms{} // reset
	// Use pointer-based deduplication (already done at unmarshal time)
	if len(mergedData.TrustedSources) == 0 {
		// No trusted sources reported, return empty state
		log.WithContext(ctx).Debugf("TrustedSourcesHandler.ProcessDataFromCentralData no trusted sources reported, returning empty state")
		return ts.ProcessData(ctx, state)
	}
	// Create trusted sources lookup map for O(1) checks - dereference pointers
	trustedMap := make(map[string]bool, len(mergedData.TrustedSources))
	for _, srcPtr := range mergedData.TrustedSources {
		trustedMap[*srcPtr] = true
	}

	for key, entry := range mergedData.Logger {
		if !containsTrustedSourcePtrs(entry.TotalSources, trustedMap) {
			// skip if no trusted source reported this key
			continue
		}
		if ts.id.Type == models.TypesTrusted {
			for typeKey, sourcePtrs := range entry.Types {
				// check if any source is trusted else skip
				for _, srcPtr := range sourcePtrs {
					src := *srcPtr
					// Only add src if it was reported in trusted sources
					if trustedMap[src] {
						if _, ok := ts.trustedLog[key]; !ok {
							ts.trustedLog[key] = map[string]map[string]bool{}
						}
						if _, ok := ts.trustedLog[key][typeKey]; !ok {
							ts.trustedLog[key][typeKey] = map[string]bool{}
						}
						log.WithContext(ctx).Debugf("TrustedSourcesHandler.ProcessDataFromCentralData adding src: %v to key: %v typeKey: %v", src, key, typeKey)
						ts.trustedLog[key][typeKey][src] = true
					}
				}
			}
		} else {
			for indKey, sourcePtrs := range entry.Indicators {
				for _, srcPtr := range sourcePtrs {
					src := *srcPtr
					// Only add src if it was reported in trusted sources
					if trustedMap[src] {
						if _, ok := ts.trustedLog[key]; !ok {
							ts.trustedLog[key] = map[string]map[string]bool{}
						}
						if _, ok := ts.trustedLog[key][indKey]; !ok {
							ts.trustedLog[key][indKey] = map[string]bool{}
						}
						log.WithContext(ctx).Debugf("TrustedSourcesHandler.ProcessDataFromCentralData adding src: %v to key: %v indKey: %v", src, key, indKey)
						ts.trustedLog[key][indKey][src] = true
					}
				}
			}
		}
	}
	// Return processed state
	return ts.ProcessData(ctx, state)
}

// ProcessData gets the last state and update the state according to the collected data
func (ts *TrustedSourcesHandler) ProcessData(ctx context.Context, stateIfs models.State) models.State {
	log.WithContext(ctx).Debugf("TrustedSourcesHandler.ProcessData ids: %+v", ts.id)
	// Handle nil state - treat it as empty state
	var existingState *trustedSourcesState
	if stateIfs == nil {
		log.WithContext(ctx).Debugf("TrustedSourcesHandler.ProcessData received nil state, using empty state")
		existingState = &trustedSourcesState{[]mapToMapToSet{}}
	} else {
		existingState = stateIfs.(*trustedSourcesState)
	}
	// merge the state with the trusted log
	mergeAndConvertCerealToGo(existingState.Logger, ts.trustedLog, false)
	newState := &trustedSourcesState{[]mapToMapToSet{}}
	for key, keyData := range ts.trustedLog {
		keyObj := mapToMapToSet{Key: key}
		for value, sources := range keyData {
			valueObj := mapToSet{Key: value}
			for source := range sources {
				valueObj.Value = append(valueObj.Value, source)
			}
			keyObj.Value = append(keyObj.Value, valueObj)
		}
		newState.Logger = append(newState.Logger, keyObj)
	}
	return newState
}

// GetDependencies return the handlers that this handler is dependent on
func (ts *TrustedSourcesHandler) GetDependencies() map[models.SyncID]models.SyncHandler {
	return nil
}
