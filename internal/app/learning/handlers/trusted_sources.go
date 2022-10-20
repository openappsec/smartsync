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

	"openappsec.io/smartsync-service/models"
)

//TrustedSourcesHandler represents a trusted sources handler
type TrustedSourcesHandler struct {
	trustedLog mms
}

type trustedSourcesState struct {
	Logger []mapToMapToSet `json:"logger"`
}

//GetFilePath returns the path to the state file saved by the service
func (ts *trustedSourcesState) GetFilePath(id models.SyncID) string {
	return fmt.Sprintf("/%v/%v/%v/remote/data.data", id.TenantID, id.AssetID, id.Type)
}

//GetOriginalPath returns the path to the state file saved by the agent
func (ts *trustedSourcesState) GetOriginalPath(id models.SyncID) string {
	return fmt.Sprintf("/%v/%v/%v/processed/data.data", id.TenantID, id.AssetID, id.Type)
}

func (ts *trustedSourcesState) ShouldRebase() bool {
	return false
}

//NewTrustedSources creates new trusted sources handler
func NewTrustedSources() *TrustedSourcesHandler {
	return &TrustedSourcesHandler{trustedLog: mms{}}
}

// NewDataStruct returns a struct representation of the data put by agent
func (ts *TrustedSourcesHandler) NewDataStruct() interface{} {
	return &trustedSourcesState{[]mapToMapToSet{}}
}

// MergeData merges the data from agent into struct member
func (ts *TrustedSourcesHandler) MergeData(data interface{}) {
	state := data.(*trustedSourcesState)
	mergeAndConvertCerealToGo(state.Logger, ts.trustedLog)
}

//NewState returns a struct representing the state
func (ts *TrustedSourcesHandler) NewState() models.State {
	return &trustedSourcesState{[]mapToMapToSet{}}
}

// SetCompressionEnabled set a flag that compression is supported
func (ts *TrustedSourcesHandler) SetCompressionEnabled() {
	// do nothing
}

//ProcessData gets the last state and update the state according to the collected data
func (ts *TrustedSourcesHandler) ProcessData(ctx context.Context, _ models.State) models.State {
	state := &trustedSourcesState{[]mapToMapToSet{}}
	for key, keyData := range ts.trustedLog {
		keyObj := mapToMapToSet{Key: key}
		for value, sources := range keyData {
			valueObj := mapToSet{Key: value}
			for source := range sources {
				valueObj.Value = append(valueObj.Value, source)
			}
			keyObj.Value = append(keyObj.Value, valueObj)
		}
		state.Logger = append(state.Logger, keyObj)
	}
	return state
}

//GetDependencies return the handlers that this handler is dependent on
func (ts *TrustedSourcesHandler) GetDependencies() map[models.SyncID]models.SyncHandler {
	return nil
}
