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
	"math"
	"strconv"
	"strings"
	"time"

	"openappsec.io/smartsync-service/models"
	"openappsec.io/errors"
	"openappsec.io/log"
)

const (
	benignParamMinRatioValue = 0.25

	keySuffixBase64 = ".#base64"
	keySuffixHTML   = ".html"
)

// Repository defines interface to a repository
type Repository interface {
	PostFile(ctx context.Context, tenantID string, path string, compress bool, data interface{}) error
}

//NewConfidenceCalculator creates a new struct of confidence calculator
func NewConfidenceCalculator(id models.SyncID, params models.ConfidenceParams, tuningDecisions models.TuningEvents, repo Repository) *ConfidenceCalculator {
	cc := &ConfidenceCalculator{
		params:    params,
		decisions: tuningDecisions,
		id:        id,
		repo:      repo,
		md:        confidenceMergedData{log: mms{}},
	}
	if id.Type == models.IndicatorsConfidence {
		scannersID := models.SyncID{
			TenantID: id.TenantID,
			AssetID:  id.AssetID,
			Type:     models.ScannersDetector,
			WindowID: id.WindowID,
		}
		cc.filterSources = NewScannersDetectorHandler()
		cc.dependencies = map[models.SyncID]models.SyncHandler{scannersID: cc.filterSources}
	}
	return cc
}

//ConfidenceCalculator represents a confidence calculator handler
type ConfidenceCalculator struct {
	dependencies  map[models.SyncID]models.SyncHandler
	filterSources *ScannersDetectorHandler
	md            confidenceMergedData
	params        models.ConfidenceParams
	decisions     models.TuningEvents
	repo          Repository
	id            models.SyncID
	compressData  bool
}

type mapToSet struct {
	Key   string   `json:"key"`
	Value []string `json:"value"`
}

type mapToMapToSet struct {
	Key   string     `json:"key"`
	Value []mapToSet `json:"value"`
}

type logger struct {
	WindowLogger []mapToMapToSet `json:"window_logger"`
}

type confidenceMergedData struct {
	// maps key -> value -> sources set
	log mms
}

// Values list of valuesSet
type valuesSet []string

// ValuesWithTime a pair of valuesSet and last modified unix timestamp
type ValuesWithTime struct {
	First  valuesSet `json:"first"`
	Second int64     `json:"second"`
}

// mapKeyValue cereal representation of a map for confidence set
type mapKeyValue struct {
	Key   string         `json:"key"`
	Value ValuesWithTime `json:"value"`
}

// valueWithLevel a pair of valuesSet and confidence level
type valueWithLevel struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
}

// mapKeyValueLevel cereal representation of a map for confidence level
type mapKeyValueLevel struct {
	Key   string           `json:"key"`
	Value []valueWithLevel `json:"value"`
}

// confidenceData the content of confidence file generated by the agent
type confidenceData struct {
	Version          int                `json:"version,omitempty"`
	ConfidenceSet    []mapKeyValue      `json:"confidence_set"`
	ConfidenceLevels []mapKeyValueLevel `json:"confidence_levels"`
}

func (cd *confidenceData) ShouldRebase() bool {
	return cd.Version == 0
}

func (cd *confidenceData) GetOriginalPath(ids models.SyncID) string {
	return fmt.Sprintf("/%v/%v/%v/processed/confidence.data", ids.TenantID, ids.AssetID, ids.Type)
}

//GetFilePath returns the path to the state file saved by the service
func (cd *confidenceData) GetFilePath(id models.SyncID) string {
	return fmt.Sprintf("/%v/%v/%v/remote/confidence.data", id.TenantID, id.AssetID, id.Type)
}

type confidenceDataMaps struct {
	confidenceSet    map[string]map[string]bool
	confidenceLevels map[string]map[string]float64
}

// NewDataStruct returns a struct representation of the data put by agent
func (c *ConfidenceCalculator) NewDataStruct() interface{} {
	return &logger{WindowLogger: []mapToMapToSet{}}
}

// SetCompressionEnabled set a flag that compression is supported
func (c *ConfidenceCalculator) SetCompressionEnabled() {
	c.compressData = true
}

// MergeData merges the data from agent into struct member
func (c *ConfidenceCalculator) MergeData(data interface{}) {
	logData := data.(*logger)
	for _, key := range logData.WindowLogger {
		if _, ok := c.md.log[key.Key]; !ok {
			c.md.log[key.Key] = map[string]map[string]bool{}
		}
		for _, value := range key.Value {
			if _, ok := c.md.log[key.Key][value.Key]; !ok {
				c.md.log[key.Key][value.Key] = map[string]bool{}
			}
			for _, src := range value.Value {
				c.md.log[key.Key][value.Key][src] = true
			}
		}
	}
}

//NewState returns a struct representing the state
func (c *ConfidenceCalculator) NewState() models.State {
	return &confidenceData{}
}

func convertStateToModel(state *confidenceData) confidenceDataMaps {
	stateModel := confidenceDataMaps{
		confidenceSet:    map[string]map[string]bool{},
		confidenceLevels: map[string]map[string]float64{},
	}
	if state == nil {
		return stateModel
	}
	for _, key := range state.ConfidenceSet {
		if _, ok := stateModel.confidenceSet[key.Key]; !ok {
			stateModel.confidenceSet[key.Key] = map[string]bool{}
		}
		for _, value := range key.Value.First {
			stateModel.confidenceSet[key.Key][value] = true
		}
	}
	for _, key := range state.ConfidenceLevels {
		if _, ok := stateModel.confidenceLevels[key.Key]; !ok {
			stateModel.confidenceLevels[key.Key] = map[string]float64{}
		}
		for _, value := range key.Value {
			trimmedKey := strings.TrimSpace(value.Key)
			if val, ok := stateModel.confidenceLevels[key.Key][trimmedKey]; ok {
				if val < value.Value {
					stateModel.confidenceLevels[key.Key][trimmedKey] = value.Value
				}
			}
			stateModel.confidenceLevels[key.Key][value.Key] = value.Value
		}
	}
	return stateModel
}

func convertToDataStruct(key string, valuesAndSources map[string]map[string]bool) mapToMapToSet {
	ret := mapToMapToSet{
		Key:   key,
		Value: nil,
	}
	for value, sources := range valuesAndSources {
		if len(sources) == 0 {
			continue
		}
		valueSourceSet := mapToSet{
			Key:   value,
			Value: []string{},
		}
		for source := range sources {
			valueSourceSet.Value = append(valueSourceSet.Value, source)
		}
		ret.Value = append(ret.Value, valueSourceSet)
	}
	return ret
}

//ProcessData gets the last state and update the state according to the collected data
func (c *ConfidenceCalculator) ProcessData(ctx context.Context, state models.State) models.State {
	stateStruct := state.(*confidenceData)
	stateModel := convertStateToModel(stateStruct)

	log.WithContext(ctx).Debugf("confidence calculator state model: %v", stateModel)
	carryOnData := c.NewDataStruct().(*logger)

	for key, values := range c.md.log {
		isKeyBenign := c.isParamBenign(key)
		if len(values) == 1 {
			log.WithContext(ctx).Debugf("nothing to process key: %v", key)
			continue
		}
		allSources := values[c.params.NullObject]
		hasSourcesToFilter := false
		if c.filterSources != nil {
			hasSourcesToFilter = c.filterSources.filter(allSources)
			c.md.log[key][c.params.NullObject] = allSources
		}
		totalSources := c.countSources(ctx, allSources)
		if totalSources < c.params.MinSources {
			log.WithContext(ctx).Debugf("not enough sources for key: %v, sources: %v", key, allSources)
			if hasSourcesToFilter {
				if totalSources == 0 {
					// all sources on this key were marked suspicious
					continue
				}
				for value, sources := range values {
					if value == c.params.NullObject {
						continue
					}
					if c.filterSources != nil {
						c.filterSources.filter(sources)
						c.md.log[key][value] = sources
					}
				}
			}
			carryOnData.WindowLogger = append(carryOnData.WindowLogger, convertToDataStruct(key, values))
			continue
		}
		for value, sources := range values {
			if value == c.params.NullObject {
				continue
			}
			if _, ok := stateModel.confidenceLevels[key]; !ok {
				stateModel.confidenceLevels[key] = map[string]float64{}
			}
			if _, ok := stateModel.confidenceLevels[key][value]; !ok {
				stateModel.confidenceLevels[key][value] = 0.0
			}
			if c.filterSources != nil && !isKeyBenign {
				c.filterSources.filter(sources)
				c.md.log[key][value] = sources
			}

			confidenceDelta := c.calculateConfidenceDelta(ctx, sources, totalSources, isKeyBenign)
			if isKeyBenign {
				log.WithContext(ctx).Debugf("parameter %v is set to benign", key)
				confidenceDelta *= 2
			}

			log.WithContext(ctx).Debugf(
				"key: %v, value: %v, confidence delta: %v, sources: %v, total sources: %v",
				key, value, confidenceDelta, sources, totalSources,
			)

			stateModel.confidenceLevels[key][value] += confidenceDelta
			if stateModel.confidenceLevels[key][value] >= 100.0 {
				if _, ok := stateModel.confidenceSet[key]; !ok {
					stateModel.confidenceSet[key] = map[string]bool{value: true}
				} else {
					stateModel.confidenceSet[key][value] = true
				}
			}
		}
	}

	// reduce confidence on value not seen in this time frame but other were seen
	for key, values := range stateModel.confidenceLevels {
		if _, ok := c.md.log[key]; !ok {
			continue
		}
		if c.isParamBenign(key) {
			continue
		}
		for value, level := range values {
			if _, ok := c.md.log[key][value]; !ok {
				stateModel.confidenceLevels[key][value] = level * c.params.RatioThreshold
			}
		}
	}

	if len(carryOnData.WindowLogger) > 0 {
		nextWindowID, err := c.getNextWindowID()
		if err != nil {
			log.WithContext(ctx).Errorf("failed to post carry on data, err: %v", err)
		} else {
			path := fmt.Sprintf("/%v/%v/%v/%v/msrv/carry-on.data", c.id.TenantID, c.id.AssetID, c.id.Type, nextWindowID)
			err = c.repo.PostFile(context.Background(), c.id.TenantID, path, c.compressData, carryOnData)
			if err != nil {
				log.WithContext(ctx).Errorf("failed to post carry on data, err: %v", err)
			} else {
				log.WithContext(ctx).Infof("posted carry on data to: %v", path)
			}
		}
	}
	newState := convertModelToState(stateModel)
	return newState
}

func convertModelToState(model confidenceDataMaps) *confidenceData {
	state := &confidenceData{
		Version:          1,
		ConfidenceSet:    []mapKeyValue{},
		ConfidenceLevels: []mapKeyValueLevel{},
	}
	for key, keyData := range model.confidenceLevels {
		if len(keyData) == 0 {
			continue
		}
		keyState := mapKeyValueLevel{Key: key}
		if strings.HasSuffix(key, keySuffixHTML) {
			keyData = mergeKeyData(keyData, model.confidenceLevels[key[:len(key)-len(keySuffixHTML)]])
		} else if strings.HasSuffix(key, keySuffixBase64) {
			keyData = mergeKeyData(keyData, model.confidenceLevels[key[:len(key)-len(keySuffixBase64)]])
		}
		for value, level := range keyData {
			trimmedValue := strings.TrimSpace(value)
			if trimVal, ok := keyData[trimmedValue]; ok {
				if trimVal > level {
					level = trimVal
				}
			}
			valueLevelPair := valueWithLevel{
				Key:   value,
				Value: level,
			}
			keyState.Value = append(keyState.Value, valueLevelPair)
		}
		state.ConfidenceLevels = append(state.ConfidenceLevels, keyState)
	}

	for key, keyData := range model.confidenceSet {
		if len(keyData) == 0 {
			continue
		}
		keyState := mapKeyValue{Key: key}
		valueLevelPair := ValuesWithTime{
			First:  valuesSet{},
			Second: 0,
		}
		for value := range keyData {
			if len(value) == 0 {
				continue
			}
			valueLevelPair.First = append(valueLevelPair.First, value)
		}
		keyState.Value = valueLevelPair
		state.ConfidenceSet = append(state.ConfidenceSet, keyState)
	}
	return state
}

func mergeKeyData(orig map[string]float64, other map[string]float64) map[string]float64 {
	for key, val := range other {
		if base, ok := orig[key]; ok && base > val {
			continue
		}
		orig[key] = val
	}
	return orig
}

func (c *ConfidenceCalculator) calculateConfidenceDelta(
	ctx context.Context,
	sources map[string]bool,
	totalSources int,
	isKeyBenign bool) float64 {
	rationThreshold := c.params.RatioThreshold
	minSources := float64(c.params.MinSources)
	minIntervals := float64(c.params.MinIntervals)

	if len(sources) == 0 {
		return -math.Ceil(100.0 / minIntervals)
	}

	sourcesCount := float64(c.countSources(ctx, sources))

	ratio := sourcesCount / float64(totalSources)

	if isKeyBenign {
		if ratio < benignParamMinRatioValue {
			ratio = benignParamMinRatioValue
		}
		if sourcesCount == 1 {
			sourcesCount++
		}
	}

	delta := math.Ceil(100.0/minIntervals) * (ratio / rationThreshold) *
		(math.Log(sourcesCount) / math.Log(minSources))

	return delta
}

//GetDependencies return the handlers that this handler is dependent on
func (c *ConfidenceCalculator) GetDependencies() map[models.SyncID]models.SyncHandler {
	return c.dependencies
}

func (c *ConfidenceCalculator) isParamBenign(key string) bool {
	keySplit := strings.SplitN(key, "#", 2)
	param := key
	if len(keySplit) == 2 {
		param = keySplit[1]
	}

	return c.isBenignOfType(param, models.EventTypeParam)
}

func (c *ConfidenceCalculator) isBenignOfType(title string, eventType models.EventTypeEnum) bool {
	for _, tuneEvent := range c.decisions.Decisions {
		if tuneEvent.EventType == eventType &&
			tuneEvent.EventTitle == title &&
			tuneEvent.Decision == "benign" {
			return true
		}
	}
	return false
}

func (c *ConfidenceCalculator) countSources(ctx context.Context, sources map[string]bool) int {
	count := 0
	for source := range sources {
		if c.isSourceBenign(source) {
			log.WithContext(ctx).Debugf("source %v is set to benign", source)
			count += c.params.MinSources
		} else {
			count++
		}
	}
	return count
}

func (c *ConfidenceCalculator) isSourceBenign(source string) bool {
	return c.isBenignOfType(source, models.EventTypeSource)
}

func (c *ConfidenceCalculator) getNextWindowID() (string, error) {
	idSplit := strings.Split(c.id.WindowID, "_")
	if len(idSplit) != 3 {
		return "", errors.Errorf("failed to generate next window for carry on from: %v", c.id.WindowID)
	}
	dayIndex, _ := strconv.Atoi(idSplit[1])
	intervalIndex, _ := strconv.Atoi(idSplit[2])
	intervalIndex++
	if int(time.Hour*24/c.params.Interval) <= intervalIndex {
		dayIndex++
		intervalIndex = 0
	}
	return fmt.Sprintf("window_%v_%v", dayIndex, intervalIndex), nil
}
