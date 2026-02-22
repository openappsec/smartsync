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

package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"openappsec.io/log"

	"openappsec.io/smartsync-service/internal/app/learning/handlers"

	"openappsec.io/errors"

	"openappsec.io/smartsync-service/models"
)

func genKey(id models.SyncID) string {
	return fmt.Sprintf("%v_%v_%v_%v_sync", id.TenantID, id.AssetID, id.Type, id.WindowID)
}

func (lc *LearnCore) unlockOnPanic(ctx context.Context, lockKey string) {
	if r := recover(); r != nil {
		log.WithContext(ctx).Errorf(
			"panic occurred while running sync worker: %v, stack: \n%v", r, string(debug.Stack()),
		)
		lc.lock.Unlock(ctx, lockKey)
	}
}

// ProcessSyncRequest process a request to sync
func (lc *LearnCore) ProcessSyncRequest(ctx context.Context, ids models.SyncID) error {
	lockKey := genKey(ids)
	hasLock := lc.lock.Lock(ctx, lockKey)
	if !hasLock {
		log.WithContext(ctx).Infof("not handling event %v, failed to acquire lock", ids)
		return nil
	}
	log.WithContext(ctx).Infof("handling event: %v", ids)
	defer lc.unlockOnPanic(ctx, lockKey)
	if ids.Type == models.CentralizedData {
		log.WithContext(ctx).Infof("handling centralized data all handlers for event: %+v", ids)
		err := lc.handleCentralDataType(ctx, ids)
		if err != nil {
			err = errors.Wrap(err, "failed running central type")
			lc.lock.Unlock(ctx, lockKey)
		}
		return err
	}
	syncHandlers, err := lc.handlersFactory(ctx, ids)
	if err != nil {
		log.WithContext(ctx).Warnf("failed to get handlers for: %v, err: %v", ids, err)
		return nil
	}
	if len(syncHandlers) == 0 {
		log.WithContext(ctx).Debugf("no handlers for event: %v", ids)
		return nil
	}
	err = lc.syncWorker(ctx, syncHandlers)
	if err != nil {
		err = errors.Wrap(err, "failed running sync worker")
		lc.lock.Unlock(ctx, lockKey)
	}
	return err
}

func (lc *LearnCore) getTuningDecisions(ctx context.Context, ids models.SyncID) models.TuningEvents {
	var events models.TuningEvents
	path := fmt.Sprintf("%s/%s/tuning/decisions.data", ids.TenantID, ids.AssetID)
	_, err := lc.repo.GetFile(ctx, ids.TenantID, path, &events)
	if err != nil {
		if !errors.IsClass(err, errors.ClassNotFound) {
			log.WithContext(ctx).Errorf("failed to get file %s, err: %v", path, err)
		} else {
			log.WithContext(ctx).Info("no decisions found")
		}
		return models.TuningEvents{}
	}
	return events
}

func (lc *LearnCore) handlersFactory(ctx context.Context, id models.SyncID) (map[models.SyncID]models.SyncHandler, error) {
	ret := map[models.SyncID]models.SyncHandler{}
	switch id.Type {
	case models.IndicatorsConfidence:
		params := models.ConfidenceParams{
			MinSources:     3,
			MinIntervals:   5,
			RatioThreshold: 0.8,
			NullObject:     "",
			Interval:       2 * time.Hour,
		}
		ret[id] = handlers.NewConfidenceCalculator(id, params, lc.getTuningDecisions(ctx, id), lc.repo)
	case models.IndicatorsTrusted:
		ret[id] = handlers.NewTrustedSources(id)
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
		ret[id] = handlers.NewConfidenceCalculator(id, params, lc.getTuningDecisions(ctx, id), lc.repo)
	case models.TypesTrusted:
		ret[id] = handlers.NewTrustedSources(id)
	case models.CentralizedData:
		// Centralized data is handled separately in ProcessSyncRequest
		// do nothing here
	default:
		return nil, errors.Errorf("type %v is unrecognized", id.Type)
	}
	return ret, nil
}

func (lc *LearnCore) handleCentralDataType(ctx context.Context, ids models.SyncID) error {
	log.WithContext(ctx).Infof("handleCentralDataType id: %+v", ids)
	// create central data collector
	centralDataCollector := handlers.NewCentralDataCollector(ids, lc.repo)
	isCompressEnable, err := lc.mergeAgentFiles(ctx, ids, centralDataCollector)
	if err != nil {
		return errors.Wrapf(err, "failed merging central agent data for %s", ids.TenantID)
	}

	allHandlers := centralDataCollector.GetAllHandlers(ctx, ids, lc.getTuningDecisions(ctx, ids))
	if len(allHandlers) == 0 {
		log.WithContext(ctx).Debugf("no handlers for centralized data for event: %v", ids)
		return nil
	}
	for idsH, handler := range allHandlers {
		if len(handler.GetDependencies()) > 0 {
			log.WithContext(ctx).Infof("handle dependency of: %v", idsH)
			dependencyHandler := handler.GetDependencies()
			for depIds, depHandler := range dependencyHandler {
				err := lc.invokeHandler(ctx, depHandler, isCompressEnable, depIds, centralDataCollector, ids)
				if err != nil {
					return err
				}
			}
		}
		err := lc.invokeHandler(ctx, handler, isCompressEnable, idsH, centralDataCollector, ids)
		if err != nil {
			return err
		}
	}
	// Clear references to allow GC to reclaim memory
	centralDataCollector.ClearMergedData()
	return nil
}

func (lc *LearnCore) invokeHandler(
	ctx context.Context,
	handler models.SyncHandler,
	isCompressEnable bool,
	idsH models.SyncID,
	centralDataCollector *handlers.CentralDataCollector,
	ids models.SyncID) error {

	if isCompressEnable {
		handler.SetCompressionEnabled()
	}
	state, statePath, err := lc.getState(ctx, idsH, handler)
	if err != nil {
		return errors.Wrapf(err, "failed to get state for handler: %v", idsH)
	}
	// process data from central data collector
	log.WithContext(ctx).Infof("processing centralized data for %+v", idsH)

	// merged data - new structure after merge
	state = handler.ProcessDataFromCentralData(ctx, state, centralDataCollector.GetData())
	log.WithContext(ctx).Infof("posting new state for %+v to path %s", idsH, statePath)
	// post new state
	err = lc.repo.PostFile(ctx, ids.TenantID, statePath, isCompressEnable, state)
	if err != nil {
		return errors.Wrapf(err, "failed to post new state to: %v", statePath)
	}
	return nil
}

func (lc *LearnCore) getState(ctx context.Context, ids models.SyncID, handler models.SyncHandler) (models.State, string, error) {
	log.WithContext(ctx).Infof("getting state for: %+v", ids)
	state := handler.NewState()
	statePath := state.GetFilePath(ids)
	_, err := lc.repo.GetFile(ctx, ids.TenantID, statePath, state)
	if err != nil {
		if !errors.IsClass(err, errors.ClassNotFound) {
			return nil, "", errors.Wrapf(err, "failed to get state from: %v", statePath)
		}
		return state, statePath, nil
	}
	log.WithContext(ctx).Debugf("got state: %.512v", fmt.Sprintf("%+v", state))
	if state.ShouldRebase() {
		log.WithContext(ctx).Infof("Rebasing state for %+v", ids)
		// if should rebase is true then original path must not be empty
		origStatePath := state.GetOriginalPath(ids)
		rebasedState := handler.NewState()
		_, err = lc.repo.GetFile(ctx, ids.TenantID, origStatePath, rebasedState)
		if err != nil {
			log.WithContext(ctx).Warnf("Failed to rebase state")
		} else {
			state = rebasedState
		}
	}
	return state, statePath, nil
}

func (lc *LearnCore) mergeAgentFiles(
	ctx context.Context,
	ids models.SyncID,
	dataCollector models.DataCollector,
) (bool, error) {
	isCompressEnable := false
	files, err := lc.repo.GetFilesList(ctx, ids)
	if err != nil {
		return isCompressEnable, errors.Wrap(err, "failed to get files list")
	}
	log.WithContext(ctx).Infof("merging files: %v", files)
	if len(files) == 0 {
		log.WithContext(ctx).Infof("no files to merge for: %v", ids)
		return isCompressEnable, errors.Errorf("no files to merge for: %v", ids).SetClass(errors.ClassNotFound)
	}
	for _, file := range files {
		data := dataCollector.NewDataStruct()
		fileIsCompressed, err := lc.repo.GetFile(ctx, ids.TenantID, file, data)
		if err != nil {
			return isCompressEnable, errors.Wrapf(err, "failed to get file: %v", file)
		}
		if fileIsCompressed {
			isCompressEnable = true
		}
		dataCollector.MergeData(data)
	}
	return isCompressEnable, nil
}

func (lc *LearnCore) syncWorker(ctx context.Context, syncHandlers map[models.SyncID]models.SyncHandler) error {
	if len(syncHandlers) == 0 {
		return errors.New("got empty handlers list")
	}
	for ids, handler := range syncHandlers {
		dependenciesHandlers := handler.GetDependencies()
		if len(dependenciesHandlers) > 0 {
			log.WithContext(ctx).Infof("handle dependency of: %v", ids)
			err := lc.syncWorker(ctx, dependenciesHandlers)
			if err != nil {
				return errors.Wrap(err, "failed to sync dependency")
			}
		}
		err := lc.syncWorkerSingleHandler(ctx, ids, handler)
		if err != nil {
			return errors.Wrapf(err, "failed to sync for: %v", ids)
		}
	}
	return nil
}

func (lc *LearnCore) syncWorkerSingleHandler(ctx context.Context, ids models.SyncID, handler models.SyncHandler) error {
	log.WithContext(ctx).Debugf("running sync worker for: %+v", ids)
	isCompressEnable := false
	state := handler.NewState()
	statePath := state.GetFilePath(ids)
	_, err := lc.repo.GetFile(ctx, ids.TenantID, statePath, state)
	if err != nil {
		if !errors.IsClass(err, errors.ClassNotFound) {
			return errors.Wrapf(err, "failed to get state from: %v", statePath)
		}
		go lc.copyAgentState(ctx, ids, handler, state)
		return nil
	}
	log.WithContext(ctx).Debugf("got state: %.512v", fmt.Sprintf("%+v", state))
	if state.ShouldRebase() {
		log.WithContext(ctx).Infof("Rebasing state for %+v", ids)
		// if should rebase is true then original path must not be empty
		origStatePath := state.GetOriginalPath(ids)
		rebasedState := handler.NewState()
		_, err = lc.repo.GetFile(ctx, ids.TenantID, origStatePath, rebasedState)
		if err != nil {
			log.WithContext(ctx).Warnf("Failed to rebase state")
		} else {
			state = rebasedState
		}
	}
	isCompressEnable, state, err = lc.processData(ctx, ids, handler, state)
	if err != nil {
		return err
	}
	err = lc.repo.PostFile(ctx, ids.TenantID, statePath, isCompressEnable, state)
	if err != nil {
		return errors.Wrapf(err, "failed to post new state to: %v", statePath)
	}
	return nil
}

func (lc *LearnCore) processData(
	ctx context.Context,
	ids models.SyncID,
	handler models.SyncHandler,
	state models.State) (bool, models.State, error) {
	isCompressEnable := false
	files, err := lc.repo.GetFilesList(ctx, ids)
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to get files list")
	}
	log.WithContext(ctx).Infof("merging files: %v", files)
	for _, file := range files {
		data := handler.NewDataStruct()
		isCompressEnable, err = lc.repo.GetFile(ctx, ids.TenantID, file, data)
		if isCompressEnable {
			// in case of mixed compressed and not compressed - use compression
			handler.SetCompressionEnabled()
		}
		if err != nil {
			return false, nil, errors.Wrapf(err, "failed to get file: %v", file)
		}
		handler.MergeData(data)
	}

	state = handler.ProcessData(ctx, state)
	log.WithContext(ctx).Debugf("new state: %.512v", fmt.Sprintf("%+v", state))
	return isCompressEnable, state, nil
}

func (lc *LearnCore) copyAgentState(ctx context.Context, ids models.SyncID, handler models.SyncHandler, state models.State) {
	origStatePath := state.GetOriginalPath(ids)
	if origStatePath == "" {
		return
	}
	isCompressEnable, err := lc.repo.GetFile(ctx, ids.TenantID, origStatePath, state)
	if errors.IsClass(err, errors.ClassNotFound) {
		log.WithContext(ctx).Infof("no file found at: %v, processing data", origStatePath)
		isCompressEnable, state, err = lc.processData(ctx, ids, handler, state)
		if err != nil {
			log.WithContext(ctx).Errorf("Failed to process data for %+v", ids)
			return
		}
	} else if err != nil {
		log.WithContext(ctx).Errorf("failed to get state from: %v. err: %v", origStatePath, err)
		return
	}
	log.WithContext(ctx).Infof("copy state from: %v", origStatePath)

	statePath := state.GetFilePath(ids)
	err = lc.repo.PostFile(ctx, ids.TenantID, statePath, isCompressEnable, state)
	if err != nil {
		log.WithContext(ctx).Errorf("failed to post state copied from agent. err: %v", err)
	}
}

type jsonFileData struct {
	Filename string                 `json:"Filename"`
	Data     map[string]interface{} `json:"Data"`
}

type outputLearningData struct {
	Files []jsonFileData `json:"Files"`
}

// ReadS3Files Get all files for an asset/tenant ids in json form
func (lc *LearnCore) ReadS3Files(ctx context.Context, ids models.SyncID) ([]byte, error) {
	resData := outputLearningData{}
	files, err := lc.repo.GetFilesList(ctx, ids)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get files list")
	} else if len(files) == 0 {
		return nil, errors.Wrap(err, "no files found")
	}

	log.WithContext(ctx).Infof("merging files: %v", files)
	for _, file := range files {
		var data map[string]interface{}
		_, err := lc.repo.GetFile(ctx, ids.TenantID, file, &data)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get file: %v", file)
		}
		resData.Files = append(resData.Files, jsonFileData{Filename: file, Data: data})
	}

	outJSON, errMarshal := json.Marshal(resData)
	if errMarshal != nil {
		return nil, errors.Wrap(err, "Failed marshalling data")
	}

	return outJSON, nil
}
