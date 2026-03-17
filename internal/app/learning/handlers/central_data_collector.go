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
	"time"

	"openappsec.io/log"

	"openappsec.io/smartsync-service/models"
)

// CentralDataCollector implements centralized data collection logic
type CentralDataCollector struct {
	ids  models.SyncID
	repo Repository
	data *models.CentralData
}

// NewCentralDataCollector creates a new instance of CentralDataCollector
func NewCentralDataCollector(ids models.SyncID, repo Repository) *CentralDataCollector {
	log.Debugf("NewCentralDataCollector ids: %+v", ids)
	return &CentralDataCollector{
		ids:  ids,
		repo: repo,
	}
}

// NewDataStruct creates a new data structure for centralized data collection
func (c *CentralDataCollector) NewDataStruct() interface{} {
	log.Debugf("CentralDataCollector.NewDataStruct ids: %+v", c.ids)
	return &models.CentralDataWrapper{}
}

// MergeData merges the provided data into the centralized data collector
func (c *CentralDataCollector) MergeData(data interface{}) {
	log.Debugf("CentralDataCollector.MergeData ids: %+v", c.ids)
	centralData, ok := data.(*models.CentralDataWrapper)
	if !ok || centralData == nil {
		return
	}
	if c.data == nil {
		c.data = &models.CentralData{
			TrustedSources: []*string{},
			Logger:         map[string]models.LoggerEntry{},
		}
	}
	// Merge TrustedSources - use map for deduplication then convert to pointers
	trustedMap := make(map[string]*string)
	for _, srcPtr := range c.data.TrustedSources {
		trustedMap[*srcPtr] = srcPtr
	}
	for _, srcPtr := range centralData.Data.TrustedSources {
		if _, exists := trustedMap[*srcPtr]; !exists {
			trustedMap[*srcPtr] = srcPtr
		}
	}
	c.data.TrustedSources = make([]*string, 0, len(trustedMap))
	for _, srcPtr := range trustedMap {
		c.data.TrustedSources = append(c.data.TrustedSources, srcPtr)
	}

	// Merge Logger entries
	for key, entry := range centralData.Data.Logger {
		if _, ok := c.data.Logger[key]; !ok {
			c.data.Logger[key] = entry
		} else {
			// Merge LoggerEntry fields - use maps for deduplication
			existingEntry := c.data.Logger[key]

			// Merge TotalSources
			totalSourcesMap := make(map[string]*string)
			for _, srcPtr := range existingEntry.TotalSources {
				totalSourcesMap[*srcPtr] = srcPtr
			}
			for _, srcPtr := range entry.TotalSources {
				if _, exists := totalSourcesMap[*srcPtr]; !exists {
					totalSourcesMap[*srcPtr] = srcPtr
				}
			}
			totalSources := make([]*string, 0, len(totalSourcesMap))
			for _, srcPtr := range totalSourcesMap {
				totalSources = append(totalSources, srcPtr)
			}
			existingEntry.TotalSources = totalSources

			// Merge Indicators
			for indKey, indSlice := range entry.Indicators {
				if _, ok := existingEntry.Indicators[indKey]; !ok {
					existingEntry.Indicators[indKey] = indSlice
				} else {
					// Merge slices with deduplication
					indMap := make(map[string]*string)
					for _, vPtr := range existingEntry.Indicators[indKey] {
						indMap[*vPtr] = vPtr
					}
					for _, vPtr := range indSlice {
						if _, exists := indMap[*vPtr]; !exists {
							indMap[*vPtr] = vPtr
						}
					}
					merged := make([]*string, 0, len(indMap))
					for _, vPtr := range indMap {
						merged = append(merged, vPtr)
					}
					existingEntry.Indicators[indKey] = merged
				}
			}

			// Merge Types
			for typeKey, sourcesSlice := range entry.Types {
				if _, ok := existingEntry.Types[typeKey]; !ok {
					existingEntry.Types[typeKey] = sourcesSlice
				} else {
					// Merge slices with deduplication
					srcMap := make(map[string]*string)
					for _, srcPtr := range existingEntry.Types[typeKey] {
						srcMap[*srcPtr] = srcPtr
					}
					for _, srcPtr := range sourcesSlice {
						if _, exists := srcMap[*srcPtr]; !exists {
							srcMap[*srcPtr] = srcPtr
						}
					}
					merged := make([]*string, 0, len(srcMap))
					for _, srcPtr := range srcMap {
						merged = append(merged, srcPtr)
					}
					existingEntry.Types[typeKey] = merged
				}
			}

			c.data.Logger[key] = existingEntry
		}
	}
}

// ClearMergedData releases references to merged central data to free memory between runs.
func (c *CentralDataCollector) ClearMergedData() {
	c.data = nil
}

// GetData returns the collected data from the centralized data collector
func (c *CentralDataCollector) GetData() *models.CentralData {
	if c.data == nil {
		// Return empty structure instead of nil to prevent panics
		return &models.CentralData{
			TrustedSources: []*string{},
			Logger:         map[string]models.LoggerEntry{},
		}
	}
	return c.data
}

// GetAllHandlers returns a map of ids to handlers for centralized data
func (c *CentralDataCollector) GetAllHandlers(ctx context.Context, baseIds models.SyncID, tuningDecisions models.TuningEvents) map[models.SyncID]models.SyncHandler {
	log.WithContext(ctx).Debugf("CentralDataCollector.GetAllHandlers baseIds: %+v", baseIds)
	handlers := map[models.SyncID]models.SyncHandler{}
	for _, syncType := range models.SyncTypes {
		ids := baseIds
		ids.Type = syncType
		var handler models.SyncHandler
		switch syncType {
		case models.IndicatorsConfidence:
			params := models.ConfidenceParams{
				MinSources:     3,
				MinIntervals:   5,
				RatioThreshold: 0.8,
				NullObject:     "",
				Interval:       2 * time.Hour,
			}
			handler = NewConfidenceCalculator(ids, params, tuningDecisions, c.repo)
		case models.IndicatorsTrusted:
			handler = NewTrustedSources(ids)
		case models.ScannersDetector:
			// do nothing - is handled as a dependency in indicators confidence
		case models.TypesConfidence:
			params := models.ConfidenceParams{
				MinSources:     10,
				MinIntervals:   5,
				RatioThreshold: 0.8,
				NullObject:     "unknown",
				Interval:       time.Hour,
			}
			handler = NewConfidenceCalculator(ids, params, tuningDecisions, c.repo)
		case models.TypesTrusted:
			handler = NewTrustedSources(ids)
		}
		if handler != nil {
			handlers[ids] = handler
		}
	}
	return handlers
}
