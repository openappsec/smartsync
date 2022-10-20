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
	"hash/fnv"

	"openappsec.io/log"

	"openappsec.io/smartsync-service/models"
)

const maxRetention = 5

type scannersMonitor struct {
	//Monitor maps source to hit keys to set of indicators
	Monitor []mapToMapToSet
}

type scannersDetectMergedData struct {
	// maps source -> key -> indicators set
	Monitor map[string]map[string]map[string]bool
}

type scannerState struct {
	LastMonitors []scannersDetectMergedData
}

func (s *scannerState) ShouldRebase() bool {
	return false
}

func (s *scannerState) GetFilePath(id models.SyncID) string {
	return fmt.Sprintf("/%v/%v/%v/remote/state.data", id.TenantID, id.AssetID, id.Type)
}

func (s *scannerState) GetOriginalPath(ids models.SyncID) string {
	// state was not saved on remote DB in agent implementation
	return ""
}

//ScannersDetectorHandler represents a scanner detector handler
type ScannersDetectorHandler struct {
	mergedMonitors  scannersDetectMergedData
	sourcesToFilter map[string]bool
}

//NewScannersDetectorHandler creates new scanner detector handler
func NewScannersDetectorHandler() *ScannersDetectorHandler {
	return &ScannersDetectorHandler{
		mergedMonitors: scannersDetectMergedData{Monitor: map[string]map[string]map[string]bool{}},
	}
}

// NewDataStruct returns a struct representation of the data put by agent
func (s *ScannersDetectorHandler) NewDataStruct() interface{} {
	return &scannersMonitor{}
}

// SetCompressionEnabled set a flag that compression is supported
func (s *ScannersDetectorHandler) SetCompressionEnabled() {
	// do nothing
}

// MergeData merges the data from agent into struct member
func (s *ScannersDetectorHandler) MergeData(data interface{}) {
	monitor := data.(*scannersMonitor)
	mergeAndConvertCerealToGo(monitor.Monitor, s.mergedMonitors.Monitor)
}

//NewState returns a struct representing the state
func (s *ScannersDetectorHandler) NewState() models.State {
	return &scannerState{[]scannersDetectMergedData{}}
}

//ProcessData gets the last state and update the state according to the collected data
func (s *ScannersDetectorHandler) ProcessData(ctx context.Context, state models.State) models.State {
	log.WithContext(ctx).Infof("processing scanner detector")
	mergedSnapshots := scannersDetectMergedData{Monitor: map[string]map[string]map[string]bool{}}
	sdState := state.(*scannerState)
	sdState.LastMonitors = append(sdState.LastMonitors, s.mergedMonitors)
	if len(sdState.LastMonitors) > maxRetention {
		sdState.LastMonitors = sdState.LastMonitors[len(sdState.LastMonitors)-maxRetention:]
	}
	mergeStateToSnapshot(sdState, mergedSnapshots)

	s.sourcesToFilter = map[string]bool{}
	for source, keys := range mergedSnapshots.Monitor {
		if len(keys) <= 2 {
			continue
		}
		hashCounter := map[uint64]int{}
		hashf := fnv.New64()
		for _, indicators := range keys {
			hashf.Reset()
			_, err := hashf.Write([]byte(fmt.Sprint(indicators)))
			if err != nil {
				continue
			}
			hashCounter[hashf.Sum64()]++
			if hashCounter[hashf.Sum64()] >= 3 {
				log.WithContext(ctx).Infof("source: %v is suspicious", source)
				s.sourcesToFilter[source] = true
				break
			}
		}
	}
	return sdState
}

func mergeStateToSnapshot(sdState *scannerState, mergedSnapshots scannersDetectMergedData) {
	for _, snapshot := range sdState.LastMonitors {
		for source, keyData := range snapshot.Monitor {
			if _, ok := mergedSnapshots.Monitor[source]; !ok {
				mergedSnapshots.Monitor[source] = keyData
				continue
			}
			for key, indicatorsSet := range keyData {
				if _, ok := mergedSnapshots.Monitor[source][key]; !ok {
					mergedSnapshots.Monitor[source][key] = indicatorsSet
					continue
				}
				for indicator := range indicatorsSet {
					mergedSnapshots.Monitor[source][key][indicator] = true
				}
			}
		}
	}
}

//GetDependencies return the handlers that this handler is dependent on
func (s *ScannersDetectorHandler) GetDependencies() map[models.SyncID]models.SyncHandler {
	return nil
}

func (s *ScannersDetectorHandler) filter(allSources map[string]bool) bool {
	hasFilter := false
	for source := range s.sourcesToFilter {
		if _, ok := allSources[source]; ok {
			hasFilter = true
			delete(allSources, source)
		}
	}
	return hasFilter
}
