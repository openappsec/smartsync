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

package eventconsumer

import (
	"context"
	"encoding/json"

	"openappsec.io/smartsync-service/models"

	"openappsec.io/ctxutils"
	"openappsec.io/errors"
	"openappsec.io/log"
)

const notificationIDSyncLearning = "b9b9ab04-2e2a-4cd1-b7e5-2c956861fb69"

type eventSource struct {
	TenantID string `json:"tenantId"`
}

type notificationConsumerData struct {
	SyncLearnNotificationConsumers models.SyncLearnNotificationConsumers `json:"syncLearnNotificationConsumers"`
}

type eventObject struct {
	NotificationConsumerData notificationConsumerData `json:"notificationConsumerData"`
}

type eventData struct {
	NotificationID string      `json:"notificationId"`
	EventObject    eventObject `json:"eventObject"`
}

type syncLog struct {
	EventSource eventSource `json:"eventSource"`
	EventData   eventData   `json:"eventData"`
}

type notificationLog struct {
	Log syncLog `json:"log"`
}

type notificationData struct {
	Logs []notificationLog `json:"logs"`
}

func (a *Adapter) handleNotification(ctx context.Context, body []byte, headers map[string][]byte) error {
	ctx = a.headersToContextMiddleware(ctx, headers)

	log.WithContext(ctx).Debugf("handling notification: %v", string(body))
	// extract notification data into FirstRequestNotification model
	var notifData notificationData
	err := json.Unmarshal(body, &notifData)
	if err != nil {
		log.WithContext(ctx).Errorf("failed to unmarshal notification log: %v", string(body))
		return err
	}

	if len(notifData.Logs) == 0 {
		log.WithContext(ctx).Warnf("failed to extracts logs from: %v", string(body))
		return errors.New("got empty notification body or fail to parse")
	}
	for _, logData := range notifData.Logs {
		if logData.Log.EventData.NotificationID != notificationIDSyncLearning {
			log.WithContext(ctx).Debugf(
				"notification %v does not match %v", logData.Log.EventData.NotificationID, notificationIDSyncLearning,
			)
			continue
		}

		ctxWithTenantID := ctxutils.Insert(ctx, ctxutils.ContextKeyTenantID, logData.Log.EventSource.TenantID)

		consumerData := logData.Log.EventData.EventObject.NotificationConsumerData.SyncLearnNotificationConsumers
		if len(consumerData.AssetID) == 0 || len(consumerData.Type) == 0 || len(consumerData.WindowID) == 0 {
			log.WithContext(ctxWithTenantID).Warnf(
				"failed to extract data from: %v, got: %v", string(body), consumerData,
			)
			return nil
		}

		syncID := models.SyncID{
			TenantID: logData.Log.EventSource.TenantID,
			AssetID:  consumerData.AssetID,
			Type:     consumerData.Type,
			WindowID: consumerData.WindowID,
		}
		log.WithContext(ctxWithTenantID).Debugf("call process sync request for id: %v", syncID)

		// call service to update
		err = a.srv.ProcessSyncRequest(ctxWithTenantID, syncID)
		if err != nil {
			log.WithContext(ctxWithTenantID).Errorf("failed to process sync request for %v, err: %v", syncID, err)
			return err
		}
	}
	return err
}
