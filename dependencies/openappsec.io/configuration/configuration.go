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

package configuration

import (
	"context"
	"time"

	"openappsec.io/configuration/flatten"

	"github.com/spf13/cast"
	"openappsec.io/errors"
	"openappsec.io/log"
)

// Repository exposes an interface for configuration repository
type Repository interface {
	Get(key string) interface{}
	GetString(key string) string
	GetBool(key string) bool
	GetInt(key string) int
	GetDuration(key string) time.Duration
	GetAll() map[string]interface{}
	Set(key string, value interface{})
	IsSet(key string) bool
	SetMany(conf map[string]interface{})
	MergeConfig(conf map[string]interface{}) error
	LoadFromFile(fileName string, filePath string, fileType string) error
}

// Service implements the configuration service.
type Service struct {
	repo  Repository
	hooks map[string]func(value interface{}) error
}

const (
	// const variables for bootstrap config file
	bootstrapConfFilePath = "configs/"
	bootstrapConfFileName = "bootstrapConfig"
	bootstrapConfFileType = "yaml"
)

// NewConfigurationService returns a new instance of the configuration service
func NewConfigurationService(repo Repository) (*Service, error) {
	if err := repo.LoadFromFile(bootstrapConfFileName, bootstrapConfFilePath, bootstrapConfFileType); err != nil {
		return nil, errors.Wrap(err, "Failed to load bootstrap config file")
	}

	return &Service{
		repo:  repo,
		hooks: make(map[string]func(value interface{}) error),
	}, nil

}

// GetAll returns all currently set configurations
func (s *Service) GetAll() map[string]interface{} {
	return s.repo.GetAll()
}

// Get returns the value for the requested key as interface{}
func (s *Service) Get(key string) interface{} {
	return s.repo.Get(key)
}

// GetString returns value for the requested key as string
// In case the key is not set in the configuration, a 'errors.ClassNotFound' will be returned.
// In case the value which the configuration holds for the given key is not a string, an 'errors.ClassInternal' will be returned.
func (s *Service) GetString(key string) (string, error) {
	if !s.repo.IsSet(key) {
		return "", keyNotFoundError(key)
	}

	value, err := cast.ToStringE(s.repo.Get(key))
	if err != nil {
		return "", invalidDataTypeError(key)
	}

	return value, nil
}

// GetBool returns value for the requested key as boolean
// In case the key is not set in the configuration, a 'errors.ClassNotFound' will be returned.
// In case the value which the configuration holds for the given key is not a bool, an 'errors.ClassInternal' will be returned.
func (s *Service) GetBool(key string) (bool, error) {
	if !s.repo.IsSet(key) {
		return false, keyNotFoundError(key)
	}

	value, err := cast.ToBoolE(s.repo.Get(key))
	if err != nil {
		return false, invalidDataTypeError(key)
	}

	return value, nil
}

// GetInt returns value for the requested key as int and
// In case the key is not set in the configuration, a 'errors.ClassNotFound' will be returned.
// In case the value which the configuration holds for the given key is not an int, an 'errors.ClassInternal' will be returned.
func (s *Service) GetInt(key string) (int, error) {
	if !s.repo.IsSet(key) {
		return 0, keyNotFoundError(key)
	}

	value, err := cast.ToIntE(s.repo.Get(key))
	if err != nil {
		return 0, invalidDataTypeError(key)
	}

	return value, nil
}

// GetDuration returns value for the requested key as time duration
// In case the key is not set in the configuration, a 'errors.ClassNotFound' will be returned.
// In case the value which the configuration holds for the given key is not a valid duration, an 'errors.ClassInternal' will be returned.
func (s *Service) GetDuration(key string) (time.Duration, error) {
	if !s.repo.IsSet(key) {
		return 0, keyNotFoundError(key)
	}

	value, err := time.ParseDuration(s.repo.GetString(key))
	if err != nil {
		return 0, invalidDataTypeError(key)
	}

	return value, nil
}

// Set places the key and value in configuration service
func (s *Service) Set(ctx context.Context, key string, value interface{}) error {
	s.repo.Set(key, value)
	if err := s.invokeHooks(map[string]interface{}{key: value}); err != nil {
		return errors.Wrapf(err, "Failed to invoke hooks for key (%s) value (%+v)", key, value)
	}

	return nil
}

// SetMany receives a map[string]interface{} and adding provided configuration on top of the existing one
func (s *Service) SetMany(ctx context.Context, conf map[string]interface{}) error {
	for k, v := range conf {
		if err := s.Set(ctx, k, v); err != nil {
			return errors.Wrapf(err, "Failed to set key (%s) value (%s)", k, v)
		}
	}

	return nil
}

// IsSet checks if the requested key exists
func (s *Service) IsSet(key string) bool {
	return s.repo.IsSet(key)
}

// RegisterHook takes a function which will be run upon change to the value of the specified key
func (s *Service) RegisterHook(key string, hook func(value interface{}) error) {
	s.hooks[key] = hook
}

func (s *Service) invokeHooksForKey(key string, value interface{}) error {
	if hook := s.hooks[key]; hook != nil {
		log.WithContext(context.Background()).Debugf("Invoking hook for key: %s with value: %+v", key, value)
		if err := hook(value); err != nil {
			return errors.Wrapf(err, "Hook run for key (%s) failed with error", key)
		}
	}
	return nil
}

func (s *Service) invokeHooks(conf map[string]interface{}) error {
	fMap := make(map[string]interface{})
	if err := flatten.Map("", conf, ".", fMap); err != nil {
		return errors.Wrapf(err, "Failed to flatten map (%+v)", fMap)
	}

	for key, value := range fMap {
		//TODO: Add the possibility to not fail after a hook failure (and run the rest of the hooks)
		if err := s.invokeHooksForKey(key, value); err != nil {
			return errors.Wrapf(err, "Failed to invoke hook for key (%s) value (%+v)", key, value)
		}
	}

	return nil
}

// HealthCheck returns success by default.
func (s *Service) HealthCheck(ctx context.Context) (string, error) {
	checkName := "Configuration Service Health"
	return checkName, nil
}

func keyNotFoundError(key string) error {
	return errors.Errorf("Failed to get key (%s) from configuration", key).SetClass(errors.ClassNotFound)
}

func invalidDataTypeError(key string) error {
	return errors.Errorf("Invalid data type for key (%s) from configuration", key).SetClass(errors.ClassInternal)
}
