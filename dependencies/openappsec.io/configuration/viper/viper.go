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

package viper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"openappsec.io/errors"
)

// Adapter implements the viper adapter for the configuration service
type Adapter struct {
	v *viper.Viper
}

// NewViper returns a new instance of Viper
func NewViper() *Adapter {
	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return &Adapter{
		v: v,
	}
}

// Get loads the requested key from this instance of ConfigurationRepository
func (a *Adapter) Get(key string) interface{} {
	return a.v.Get(key)
}

// GetString returns a string from the configuration, if conversion to string fails an empty string is returned
func (a *Adapter) GetString(key string) string {
	return a.v.GetString(key)
}

// GetBool returns a bool from the configuration, if conversion to bool fails 'false' is returned
func (a *Adapter) GetBool(key string) bool {
	return a.v.GetBool(key)
}

// GetInt returns a int from the configuration, if conversion to int fails '0' is returned
func (a *Adapter) GetInt(key string) int {
	return a.v.GetInt(key)
}

// GetDuration returns a time duration from the configuration, if conversion to time duration fails '0' is returned
func (a *Adapter) GetDuration(key string) time.Duration {
	return a.v.GetDuration(key)
}

// GetAll loads all currently set configurations for this instance of ConfigurationRepository
func (a *Adapter) GetAll() map[string]interface{} {
	return a.v.AllSettings()
}

// Set places the key and value in viper
func (a *Adapter) Set(key string, value interface{}) {
	a.v.Set(key, value)
}

// IsSet checks if the requested key has been set for this instance of ConfigurationRepository
func (a *Adapter) IsSet(key string) bool {
	return a.v.IsSet(key)
}

// SetMany receives a map[string]interface{} and adding provided configuration on top of the existing one
func (a *Adapter) SetMany(conf map[string]interface{}) {
	for key, value := range conf {
		a.v.Set(key, value)
	}
}

// SetConfig receives a map[string]interface{} and loads it as JSON configuration overriding previous data
func (a *Adapter) SetConfig(conf map[string]interface{}) error {
	b, err := json.Marshal(conf)
	if err != nil {
		return errors.Wrapf(err, "Failed to marshal config map")
	}

	if err := a.v.ReadConfig(bytes.NewBuffer(b)); err != nil {
		return errors.Wrap(err, "Failed to read config file")
	}

	return nil
}

// MergeConfig receives a map[string]interface{} configuration and merge it with the previous data
func (a *Adapter) MergeConfig(conf map[string]interface{}) error {
	if err := a.v.MergeConfigMap(conf); err != nil {
		return errors.Wrap(err, "Failed to merge configuration")
	}

	return nil
}

// LoadFromFile loads config from file specified by path argument, from either working directory or executable directory
func (a *Adapter) LoadFromFile(fileName string, filePath string, fileType string) error {
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrapf(err, "Failed to get executable path")
	}
	exePath := filepath.Dir(ex)

	a.v.SetConfigName(fileName)
	a.v.AddConfigPath(filePath)
	a.v.AddConfigPath(fmt.Sprintf("%s/%s", exePath, filePath))
	a.v.SetConfigType(fileType)

	if err := a.v.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "Failed to read config file (%s)", filepath.Join(filePath, fileName+"."+fileType))
	}

	return nil
}
