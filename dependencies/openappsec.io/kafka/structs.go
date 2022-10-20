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

import (
	"context"
	"time"
)

// HandleFunc represents the handler functions
type HandleFunc func(ctx context.Context, body []byte, headers map[string][]byte) error

// RetryPolicy represents the retry policy for kafka consumer
type RetryPolicy struct {
	MaxAttempts     int           // default - 3 attempts, maximum - 5 attempts
	BackoffDelayMin time.Duration // default - 3 seconds, maximum - 1 minute
	BackoffDelayMax time.Duration // default - 3 seconds, maximum - 1 minute
	DLQTopic        string

	CurrentAttempt int    // This shouldn't modified!
	OriginalTopic  string // This shouldn't modified!
}

//ConsumerConfig are the config params for a consumer initialization
type ConsumerConfig struct {
	Brokers                string
	Topic                  string
	GroupID                string
	Partition              int
	Timeout                time.Duration
	MinBytes               int
	MaxBytes               int
	TLS                    bool
	CommitAfterHandlingMsg bool
	RetryPolicy            *RetryPolicy
	// true if running in one of the following namespaces:
	// dev-latest, master-latest, gem-master-latest, gem-pre-prod, pre-prod, staging, prod
	RetryEnabled bool
}

//ProducerConfig are the config params for a consumer initialization
type ProducerConfig struct {
	Brokers     string
	Topic       string
	Timeout     time.Duration
	MaxAttempts int
	TLS         bool
}
