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

	"openappsec.io/smartsync-service/models"
)

// LearnCore struct
type LearnCore struct {
	repo Repository
	lock MultiplePodsLock
}

// mockgen -destination mocks/mock_repository.go -package mocks openappsec.io/smartsync-service/internal/app/learning Repository

// Repository defines interface to a repository
type Repository interface {
	GetFile(ctx context.Context, tenantID string, path string, out interface{}) (bool, error)
	GetFileRaw(ctx context.Context, tenantID string, path string) ([]byte, bool, error)
	PostFile(ctx context.Context, tenantID string, path string, compress bool, data interface{}) error
	GetFilesList(ctx context.Context, id models.SyncID) ([]string, error)
}

// MultiplePodsLock defines a distributed lock
type MultiplePodsLock interface {
	Lock(ctx context.Context, key string) bool
	Unlock(ctx context.Context, key string) error
}

// NewLearnService returns a new instance of a demo service.
func NewLearnService(repository Repository, lock MultiplePodsLock) (*LearnCore, error) {
	return &LearnCore{repo: repository, lock: lock}, nil
}
