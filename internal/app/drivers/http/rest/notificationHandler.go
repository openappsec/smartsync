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
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-uuid"
	"openappsec.io/smartsync-service/models"
	"openappsec.io/ctxutils"
	"openappsec.io/errors"
	"openappsec.io/httputils/responses"
	"openappsec.io/log"
)

// NotifySync handle sync notification event
func (a *Adapter) NotifySync(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	id, err := uuid.GenerateUUID()
	if err != nil {
		log.WithContext(ctx).Warnf("No traceID, could not generate a new uuid, got error: %v", err)
	}
	ctx = ctxutils.Insert(ctx, ctxutils.ContextKeyEventTraceID, id)
	tenantID, ok := ctxutils.Extract(ctx, ctxutils.ContextKeyTenantID).(string)

	if !ok {
		errorHandling(ctx, writer, request, err, "65efb2e9-06ab-4aa4-99dc-e99b9d507502",
			"Failed to extract tenant ID")
		return
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		errorHandling(ctx, writer, request, err, "79800ff0-ba40-41fc-8960-d8910e414a4f",
			"Failed to read request body")
		return
	}

	var syncData models.SyncLearnNotificationConsumers
	err = json.Unmarshal(body, &syncData)

	if err != nil {
		errorHandling(ctx, writer, request, err, "5d1a7de2-eaf2-4b20-8d77-103c74dab101",
			"Failed to unmarshal request body")
		return
	}

	err = validateBody(syncData)
	if err != nil {
		log.WithContextAndEventID(ctx, "a29f1d4c-59f9-45e0-bb86-41306a105a23").
			Errorf("missing fields in body. Error: %s", err.Error())
		httpReturnError(
			ctx,
			writer,
			http.StatusBadRequest,
			request.URL.Path,
			http.StatusText(http.StatusBadRequest),
		)
		return
	}

	syncIds := models.SyncID{
		TenantID: tenantID,
		AssetID:  syncData.AssetID,
		Type:     syncData.Type,
		WindowID: syncData.WindowID,
	}

	go func(ctx context.Context) {
		err := a.appSrv.ProcessSyncRequest(ctx, syncIds)
		if err != nil {
			log.WithContextAndEventID(ctx, "8ecd9957-9eb9-4472-8fa0-6c22375c94ab").
				Errorf("failed to sync request, err: %v", err)
		}
	}(ctxutils.Detach(ctx))

	responses.HTTPReturn(ctx, writer, http.StatusOK, nil, true)
}

func validateBody(data models.SyncLearnNotificationConsumers) error {
	var err error
	if data.AssetID == "" {
		err = errors.Wrap(err, "missing asset id")
	}
	if data.WindowID == "" {
		err = errors.Wrap(err, "missing window id")
	}
	if data.Type == "" {
		err = errors.Wrap(err, "missing type")
	}
	return err
}

func errorHandling(
	ctx context.Context,
	writer http.ResponseWriter,
	request *http.Request,
	err error,
	eventID string,
	message string,
) {
	log.WithContextAndEventID(ctx, eventID).Errorf("%s. Error: %s", message, err.Error())
	httpReturnError(
		ctx,
		writer,
		http.StatusInternalServerError,
		request.URL.Path,
		http.StatusText(http.StatusInternalServerError),
	)
}

// ErrorResponse defines a REST request error response schema
type ErrorResponse struct {
	Timestamp string
	Path      string
	Code      string
	Message   string
}

func httpReturnError(
	ctx context.Context,
	writer http.ResponseWriter,
	code int,
	path string,
	msg string,
) {
	body, _ := json.Marshal(
		ErrorResponse{
			Timestamp: time.Now().UTC().Format(log.RFC3339MillisFormat),
			Path:      path,
			Code:      strconv.Itoa(code),
			Message:   msg,
		},
	)

	responses.HTTPReturn(ctx, writer, code, body, true)
}
