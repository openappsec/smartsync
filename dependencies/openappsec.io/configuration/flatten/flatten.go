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

package flatten

import "openappsec.io/errors"

// Map receive a map[string]interface{} and flatten the map to a map[string]interface{} (with delimiter as a separate between keys)
func Map(prefix string, m interface{}, delimiter string, res map[string]interface{}) error {
	switch m1 := m.(type) {
	case map[string]interface{}:
		for k, v := range m1 {
			if prefix == "" {
				if err := Map(k, v, delimiter, res); err != nil {
					return err
				}
			} else {
				if err := Map(joinKeys(prefix, k, delimiter), v, delimiter, res); err != nil {
					return err
				}
			}
		}
	case string, int, bool:
		res[prefix] = m1
	default:
		return errors.Errorf("Invalid format of map input: %v", m1)
	}

	return nil
}

// Concat two keys with delimiter (dot, path, etc.) between them.
// If one of the keys is empty, return the other one
func joinKeys(key1, key2 string, delimiter string) string {
	if key1 != "" && key2 != "" {
		return key1 + delimiter + key2
	}

	if key1 == "" {
		return key2
	}

	return key1
}
