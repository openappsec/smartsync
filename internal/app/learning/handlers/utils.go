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
	"strings"
)

// Configuration defines the get configuration interface
type Configuration interface {
	GetString(key string) (string, error)
}

// mms type defines a map to map to set.
type mms map[string]map[string]map[string]bool

func mergeAndConvertCerealToGo(src []mapToMapToSet, dst mms, shouldStrip bool) {
	for _, srcData := range src {
		if _, ok := dst[srcData.Key]; !ok {
			dst[srcData.Key] = map[string]map[string]bool{}
		}
		for _, keyData := range srcData.Value {
			key := keyData.Key
			if shouldStrip {
				key = stripSuffix(key)
			}
			if _, ok := dst[srcData.Key][key]; !ok {
				dst[srcData.Key][key] = map[string]bool{}
			}
			for _, indicator := range keyData.Value {
				dst[srcData.Key][key][indicator] = true
			}
		}
	}
}

func stripSuffix(key string) string {
	suffices := []string{".pipe", ".sem", ".amp", ".comma", ".asterisk"}
	strippedKey := ""
	for _, suffix := range suffices {
		strippedKey = strings.TrimSuffix(key, suffix)
		if strippedKey != key {
			return stripSuffix(strippedKey)
		}
	}
	return strippedKey
}
