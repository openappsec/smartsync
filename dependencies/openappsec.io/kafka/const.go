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

package kafka

const (
	kafkaDefaultNetworkProtocol    = "tcp"
	kafkaDefaultDialTimeout        = "15s"
	kafkaDefaultMinBytesPerRequest = 1e3
	kafkaDefaultMaxBytesPerRequest = 10e6
	kafkaDefaultMaxAttempts        = 30
	kafkaOpName                    = "golang-kafka"
	k8sNamespaceEnvKey             = "K8S_NAMESPACE"
)
