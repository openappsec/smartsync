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

package redis

import (
	"context"
	"time"

	"openappsec.io/errors"
	"openappsec.io/log"
	"openappsec.io/redis"
)

const (
	// configuration keys
	confKeyRedisBase               = "redis."
	confKeyRedisAddress            = confKeyRedisBase + "address"
	confKeyRedisTLSEnabled         = confKeyRedisBase + "tlsEnabled"
	confKeyRedisSentinelMasterName = confKeyRedisBase + "sentinelMasterName"
	confKeyRedisSentinelPassword   = confKeyRedisBase + "sentinelPassword"
	confKeyLockTTL                 = confKeyRedisBase + "ttl"

	lockKeyPrefix = "waap-learning_"
)

// mockgen -destination mocks/mock_redis.go -package mocks -mock_names Repository=MockRedis,Configuration=MockRedisConfiguration -source ./internal/pkg/lock/redis/redis.go Redis

// Configuration exposes an interface of configuration related actions
type Configuration interface {
	GetString(key string) (string, error)
	GetDuration(key string) (time.Duration, error)
	GetInt(key string) (int, error)
	GetBool(key string) (bool, error)
}

// Redis library
type Redis interface {
	ConnectToSentinel(ctx context.Context, c redis.SentinelConf) error
	HealthCheck(ctx context.Context) (string, error)
	SetIfNotExist(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	Delete(ctx context.Context, keys ...string) (int64, error)
	TearDown(ctx context.Context) error
}

// Adapter represents the Redis adapter for sync data storage
type Adapter struct {
	r       Redis
	prefix  string
	lockTTL time.Duration
}

// Lock acquire a lock for processing a tenant's data - returns true if successful
func (a *Adapter) Lock(ctx context.Context, key string) bool {
	isSet, err := a.r.SetIfNotExist(ctx, a.prefix+key, 1, a.lockTTL)
	if err != nil {
		log.WithContext(ctx).Errorf("failed to lock resource for tenant: %v, err: %v", key, err)
		// in case of error with the redis client allow all to get the lock to prevent blocking all processing
		return true
	}
	return isSet
}

// Unlock unlocks the key for processing a tenant
func (a *Adapter) Unlock(ctx context.Context, key string) error {
	_, err := a.r.Delete(ctx, a.prefix+key)
	return err
}

// NewAdapter returns a new Redis adapter for sync
func NewAdapter(ctx context.Context, redisAdapter Redis, config Configuration) (*Adapter, error) {
	adapt := &Adapter{r: redisAdapter}
	log.WithContext(ctx).Infoln("Get Redis Sentinel configuration")

	ttl, err := config.GetDuration(confKeyLockTTL)
	if err != nil {
		return nil, err
	}

	adapt.prefix = lockKeyPrefix
	adapt.lockTTL = ttl

	address, err := config.GetString(confKeyRedisAddress)
	if err != nil {
		return nil, err
	}

	tlsEnable, err := config.GetBool(confKeyRedisTLSEnabled)
	if err != nil {
		return nil, err
	}

	masterName, err := config.GetString(confKeyRedisSentinelMasterName)
	if err != nil {
		return nil, err
	}

	password, err := config.GetString(confKeyRedisSentinelPassword)
	if err != nil {
		return nil, err
	}

	conf := redis.SentinelConf{
		Addresses:  []string{address},
		TLSEnabled: tlsEnable,
		MasterName: masterName,
		Password:   password,
	}
	log.WithContext(ctx).Infoln("Connect Redis Sentinel configuration")

	if err := adapt.r.ConnectToSentinel(ctx, conf); err != nil {
		return nil, errors.Wrap(err, "Failed to connect to sentinel redis")
	}

	return adapt, nil
}

// TearDown gracefully ends the lifespan of a redis repository instance. Closing all connections
func (a *Adapter) TearDown(ctx context.Context) error {
	return a.r.TearDown(ctx)
}
