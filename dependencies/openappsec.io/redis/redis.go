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
	"crypto/tls"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/opentracing/opentracing-go"
	"openappsec.io/errors"
	"openappsec.io/log"
	"openappsec.io/tracer"
)

const (
	redisOpName = "golang-redis"
)

// Adapter for redis which holds the client.
type Adapter struct {
	client *redis.Client
}

// SentinelConf struct represents the sentinel redis configuration
type SentinelConf struct {
	Addresses  []string
	Password   string
	TLSEnabled bool
	MasterName string
}

// StandaloneConf struct represents the standalone redis configuration
type StandaloneConf struct {
	Address    string
	Password   string
	TLSEnabled bool
}

// NewClient returns a redis adapter.
func NewClient() *Adapter {
	return &Adapter{}
}

// ConnectToSentinel connects to sentinel redis.
func (a *Adapter) ConnectToSentinel(ctx context.Context, c SentinelConf) error {
	span := tracer.GlobalTracer().StartSpan(redisOpName)
	defer span.Finish()
	span.SetTag("operation", "ConnectToSentinel")

	log.WithContext(ctx).Infoln("Creating new Redis Sentinel client")
	var options redis.FailoverOptions
	options.SentinelAddrs = c.Addresses
	options.MasterName = c.MasterName
	options.Password = c.Password
	if c.TLSEnabled {
		options.TLSConfig = &tls.Config{}
	}
	a.client = redis.NewFailoverClient(&options)

	// make sure connection can be made
	if _, err := a.HealthCheck(ctx); err != nil {
		*a = Adapter{}
		return errors.Wrapf(err, "Connection to redis sentinel could not be made")
	}

	return nil
}

// ConnectToStandalone connects to standalone redis.
func (a *Adapter) ConnectToStandalone(ctx context.Context, c StandaloneConf) error {
	span := tracer.GlobalTracer().StartSpan(redisOpName)
	defer span.Finish()
	span.SetTag("operation", "ConnectToStandalone")

	log.WithContext(ctx).Infoln("Creating new Redis Standalone client")
	var options redis.Options
	options.Addr = c.Address
	options.Password = c.Password
	if c.TLSEnabled {
		options.TLSConfig = &tls.Config{}
	}
	a.client = redis.NewClient(&options)

	// make sure connection can be made
	if _, err := a.HealthCheck(ctx); err != nil {
		*a = Adapter{}
		return errors.Wrapf(err, "Connection to redis standalone could not be made")
	}

	return nil
}

// TearDown gracefully ends the lifespan of a redis repository instance. Closing all connections.
func (a *Adapter) TearDown(ctx context.Context) error {
	span := tracer.GlobalTracer().StartSpan(redisOpName)
	defer span.Finish()
	span.SetTag("operation", "TearDown")

	log.WithContext(ctx).Infoln("Closing Redis client connections...")
	if err := a.client.WithContext(ctx).Close(); err != nil {
		return errors.Wrap(err, "Failed to close Redis client connections").SetClass(errors.ClassInternal)
	}

	return nil
}

// HealthCheck PINGs the Redis server to check if it is accessible and alive.
func (a *Adapter) HealthCheck(ctx context.Context) (string, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "HealthCheck")

	checkName := "Redis PING Test"
	if pong, err := a.client.WithContext(ctx).Ping(ctx).Result(); err != nil || pong != "PONG" {
		if err == nil {
			err = errors.Errorf("Ping response: %s", pong)
		}
		return checkName, errors.Wrap(err, "Cannot reach Redis, failing health check").SetClass(errors.ClassInternal)
	}

	return checkName, nil
}

// HGetAll queries redis with 'HGETALL' command, and put the result in result map.
func (a *Adapter) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "HGetAll")

	res, err := a.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, errors.Wrap(err, "Redis HGetAll operation returned an error").SetClass(errors.ClassInternal)
	}

	return res, nil
}

// HSet sets value/s in hash key using 'HSET' command.
func (a *Adapter) HSet(ctx context.Context, key string, values map[string]interface{}) error {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "HSet")

	res := a.client.HSet(ctx, key, values)
	if res.Err() != nil {
		return errors.Wrap(res.Err(), "Redis HSet operation returned an error")
	}

	return nil
}

// Set sets 'key' to hold 'value' using 'SET' command with ttl.
// Zero ttl means the key has no expiration time.
func (a *Adapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "Set")

	data, err := json.Marshal(value)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal value")
	}

	res := a.client.Set(ctx, key, data, ttl)
	if res.Err() != nil {
		return errors.Wrap(res.Err(), "Redis Set operation returned an error")
	}

	return nil
}

// SetIfNotExist sets 'key' to hold 'value' if key does not exist using 'SETNX' command with ttl.
// Zero ttl means the key has no expiration time.
// first return value indicates if the key was set
func (a *Adapter) SetIfNotExist(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "SetIfNotExist")

	data, err := json.Marshal(value)
	if err != nil {
		return false, errors.Wrap(err, "Failed to marshal value")
	}

	res := a.client.SetNX(ctx, key, data, ttl)
	if res.Err() != nil {
		return false, errors.Wrap(res.Err(), "Redis SetNX operation returned an error")
	}

	return res.Val(), nil
}

// SetAddToArray sets 'key' to hold array 'value' using 'SAdd' command.
// If the record exists adds the value to the current array values, else create a new record.
// First return value indicates if the key was set
func (a *Adapter) SetAddToArray(ctx context.Context, key string, value interface{}) (int, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "SetAddToArray")

	data, err := json.Marshal(value)
	if err != nil {
		return 0, errors.Wrap(err, "Failed to marshal value")
	}

	res := a.client.SAdd(ctx, key, data)
	if res.Err() != nil {
		return 0, errors.Wrap(res.Err(), "Redis SAdd operation returned an error")
	}

	return int(res.Val()), nil
}

// GetArrayMembers get the array of values saved while doing 'SMembers'.
func (a *Adapter) GetArrayMembers(ctx context.Context, key string) ([]string, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "GetArrayMembers")

	res := a.client.SMembers(ctx, key)
	if res.Err() != nil {
		return []string{}, errors.Wrap(res.Err(), "Redis SMembers operation returned an error")
	}

	return res.Val(), nil
}

// GetStringArrayMembers get the array of values saved while doing 'SMembers' and return result for set string values.
func (a *Adapter) GetStringArrayMembers(ctx context.Context, key string) ([]string, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "GetStringArrayMembers")

	res, err := a.GetArrayMembers(ctx, key)
	if err != nil {
		return []string{}, errors.Wrap(err, "GetStringArrayMembers operation returned an error")
	}

	// striping all the "" from the string values
	for i, item := range res {
		res[i] = item[1 : len(item)-1]
	}

	return res, nil
}

// Get queries redis with with 'GET' command and returns the result.
func (a *Adapter) Get(ctx context.Context, key string) (string, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "Get")

	res, err := a.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", errors.Wrapf(err, "key (%s) doesn't exist", key).SetClass(errors.ClassNotFound)
	} else if err != nil {
		return "", errors.Wrap(err, "Redis Get operation returned an error").SetClass(errors.ClassInternal)
	}

	return res, nil
}

// GetKeysByPattern queries redis with 'KEYS' command, return all keys that match the received pattern.
// if the pattern doesn't match any key, return not found error.
func (a *Adapter) GetKeysByPattern(ctx context.Context, keysPattern string) ([]string, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "GetKeysByPattern")

	res, err := a.client.Keys(ctx, keysPattern).Result()
	if err != nil {
		return nil, errors.Wrap(err, "Redis KEYS operation returned an error").SetClass(errors.ClassInternal)
	}

	if len(res) < 1 {
		return nil, errors.Errorf("pattern (%s) doesn't match any key in cache", keysPattern).SetClass(errors.ClassNotFound)
	}

	return res, nil
}

// GetAllValues queries redis with with 'MGET' command and returns a list of values at the specified keys.
// if a given key is not exist, it's ignored.
func (a *Adapter) GetAllValues(ctx context.Context, keys ...string) ([]string, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "GetAllValues")

	res, err := a.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, errors.Wrap(err, "Redis MGET operation returned an error").SetClass(errors.ClassInternal)
	}

	strSliceRes := make([]string, 0, len(res))
	for _, val := range res {
		// ignore not found keys
		if val == nil {
			continue
		}
		if strVal, ok := val.(string); ok {
			strSliceRes = append(strSliceRes, strVal)
		} else {
			log.WithContext(ctx).Warnf("GetAllValues operation failed to convert a value (%v) to string, skipping this value", val)
		}
	}

	return strSliceRes, nil
}

// Delete Delete the given keys from redis, ignore if not exist.
func (a *Adapter) Delete(ctx context.Context, keys ...string) (int64, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "DEL")

	res, err := a.client.Del(ctx, keys...).Result()
	if err != nil {
		return int64(0), errors.Wrap(err, "Redis DEL operation returned an error").SetClass(errors.ClassInternal)
	}

	return res, nil
}

// DeleteByPattern deletes the keys that match the given pattern from redis, ignore if not exist.
// returns the number of keys found and number of keys deleted.
func (a *Adapter) DeleteByPattern(ctx context.Context, pattern string) (int64, int64, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "DeleteByPattern")

	var totalCount, delCount int64
	iter := a.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		totalCount++
		err := a.client.Del(ctx, iter.Val()).Err()
		if err != nil {
			log.WithContext(ctx).Debugf("Redis DEL operation (key = %s) returned an error, skipping key: %s", iter.Val(), err.Error())
			continue
		}
		delCount++
	}
	if err := iter.Err(); err != nil {
		return int64(0), int64(0), errors.Wrap(err, "Redis SCAN iterator returned an error").SetClass(errors.ClassInternal)
	}

	return totalCount, delCount, nil
}

// Incr Increments the number stored at key by one. If the key does not exist, it is set to 0 before performing the operation.
// Returns the value of key after the increment.
func (a *Adapter) Incr(ctx context.Context, key string) (int64, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "Incr")

	res := a.client.Incr(ctx, key)
	if res.Err() != nil {
		return int64(0), errors.Wrap(res.Err(), "Redis Incr operation returned an error")
	}

	return res.Val(), nil
}

// IncrByFloat Increment the floating point stored at key by the specified value.
// If the key does not exist, it is set to 0 before performing the operation. Returns the value of key after the increment.
func (a *Adapter) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "IncrByFloat")

	res := a.client.IncrByFloat(ctx, key, value)
	if res.Err() != nil {
		return float64(0), errors.Wrap(res.Err(), "Redis IncrByFloat operation returned an error")
	}

	return res.Val(), nil
}

// WatchTransaction creates a transaction using the watch function over the receiving keys.
// The function gets a transaction function to run the watch on, and performs retry if needed as the transAttempts number.
// The function gets keys and arguments and pass them to the transaction function
func (a *Adapter) WatchTransaction(ctx context.Context, keys []string, args []string, transAttempts int, transFunc func(ctx context.Context, keys []string, args ...string) func(tx *redis.Tx) error) error {
	span, ctx := opentracing.StartSpanFromContextWithTracer(ctx, tracer.GlobalTracer(), redisOpName)
	defer span.Finish()
	span.SetTag("operation", "WatchTransaction")

	var transErr error
	for i := 0; i < transAttempts; i++ {
		err := a.client.Watch(ctx, transFunc(ctx, keys, args...), keys...)
		// success
		if err == nil {
			return nil
		}

		transErr = err
	}

	return transErr
}
