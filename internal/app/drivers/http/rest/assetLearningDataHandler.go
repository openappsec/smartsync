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

package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"openappsec.io/smartsync-service/models"

	"openappsec.io/ctxutils"
	"openappsec.io/errors"
	"openappsec.io/httputils/responses"
	"openappsec.io/log"
)

// RetrieveAssetLearningData retrieve all requested learning data for a given tenant+asset
func (a *Adapter) RetrieveAssetLearningData(writer http.ResponseWriter, request *http.Request) {

	ctx := request.Context()
	tenantID, ok := ctxutils.Extract(ctx, ctxutils.ContextKeyTenantID).(string)

	if !ok {
		errorHandling(ctx, writer, request,
			errors.Errorf("failed to cast tenant id from context to string, type: %t",
				ctxutils.Extract(ctx, ctxutils.ContextKeyTenantID)),
			"da60ba0e-dc40-478f-a760-e166a2719306",
			"Failed to extract tenant ID")
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		errorHandling(ctx, writer, request, err, "d7e5466b-9195-4c6f-b4bf-f91b5d9320a1",
			"Failed to read request body")
		return
	}

	var requestData learningData
	err = json.Unmarshal(body, &requestData)

	if err != nil {
		errorHandling(ctx, writer, request, err, "1930acd5-2e4a-4842-b060-55e6e7ff41bc",
			"Failed to unmarshal request body")
		return
	}

	err = validateData(requestData)
	if err != nil {
		log.WithContext(ctx).Errorf("missing fields in body. Error: %s", err.Error())
		httpReturnError(
			ctx,
			writer,
			http.StatusBadRequest,
			request.URL.Path,
			http.StatusText(http.StatusBadRequest),
		)
		return
	}

	out, err := a.appSrv.ReadS3Files(ctx,
		models.SyncID{
			TenantID: tenantID,
			AssetID:  requestData.AssetID,
			Type:     requestData.Type,
			WindowID: requestData.WindowID})
	if err != nil {
		errorHandling(ctx, writer, request, err, "27900368-91a2-488a-ab36-6e3f4d80e13e",
			fmt.Sprintf("Failed getting files, tenantID: %s assetID: %s",
				tenantID, requestData.AssetID))
	}

	responses.HTTPReturn(
		ctx,
		writer,
		http.StatusOK,
		out,
		true)
}

type learningData struct {
	AssetID  string          `json:"assetId"`
	Type     models.SyncType `json:"type"`
	WindowID string          `json:"windowId"`
}

func validateData(data learningData) error {
	var err error
	if data.AssetID == "" {
		err = errors.Wrap(err, "missing asset id")
	}
	if data.Type != "" {
		matched, _ := regexp.Match("^([A-Za-z]+(/[A-Za-z]+)?)$", []byte(data.Type))
		if !matched {
			err = errors.Wrap(err, "wrong type format")
		}
	}
	if data.WindowID != "" {
		matched, _ := regexp.Match("^((window_[0-9]+(_[0-9]+)?)|(remote)|(processed))$",
			[]byte(data.WindowID))
		if !matched {
			err = errors.Wrap(err, "wrong window id format")
		}
	}
	return err
}
