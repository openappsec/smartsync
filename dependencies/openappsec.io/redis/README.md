# redis

A package aimed to give convenient access to the official redis golang driver (go-redis).

## Capabilities:

### NewClient:

Creates a new, uninitialized client. To render the client usable call the `ConnectToStandalone` or `ConnectToSentinel` method. 

### ConnectToStandalone:

This function accepts a context and a struct with configuration for connection to **standalone** redis. Asserts connection with healthcheck and return error upon failed connection. 

The fields that can be passed in the configuration can be seen in the table below.<br>

| Variable         |  Type         | Description                                                                                                                                             |
|------------------|---------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| Address          | string        | the address of the redis server to which you'd like to connect.                                                                                         |
| Password         | string        | This field specifies the password with which to connect.                                                                                                |
| TLSEnabled       | boolean       | When this field set to true, TLS will be negotiated.                                                                                                    |

### ConnectToSentinel:

This function accepts a context and a struct with configuration for connection to **sentinel** redis.  Asserts connection with healthcheck and return error upon failed connection. 

The fields that can be passed in the configuration can be seen in the table below.<br>

| Variable         |  Type            | Description                                                                                                                                             |
|------------------|------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| Addresses        | slice of strings | A seed list of host:port addresses of sentinel nodes.                                                                                                   |
| Password         | string           | This field specifies the password with which to connect.                                                                                                |
| MasterName       | string           | This field specifies the master name.                                                                                                                   |
| TLSEnabled       | boolean          | When this field set to true, TLS will be negotiated.                                                                                                    |

### HealthCheck:

Exposes a healthcheck function to be used in various readiness checks. This healthcheck executes a PING operation to the redis server.

### HSet:

Accepts context, a key and a value (as a hash map) for performing redis `HSet` operation.<br>
If one (or more) of the fields of the hash map already exists, it is overwritten.<br>
Upon success a nil error is returned. 

### HGetAll:

Accepts context and a key for performing redis `HGetAll` operation.<br>
If the given key does not exist, an empty result is returned (no error is returned). <br>
Upon success the result is returned as a `map[string]string` and a nil error is returned. 

### Set:

Accepts context, a key, a value (as an `interface`) and TTL (as `time.Duration`) for performing redis `Set` operation.<br>
If key already holds a value, it is overwritten, regardless of its type.<br>
Zero TTL means the key has no expiration time.<br>
Upon success a nil error is returned. 

### SetIfNotExist:

Accepts context, a key, a value (as an `interface`) and TTL (as `time.Duration`) for performing redis `SetNX` operation.<br>
If key already holds a value, it returns false and does not override it.<br>
Zero TTL means the key has no expiration time.<br>
Upon success a true value and a nil error is returned.

### SetAddToArray:

Accepts context, a key, a value (as an `interface`) for performing redis `SAdd` operation.<br>
If the key exists adds the value to the current array values, else create a new record.<br>
Upon success returns the number of set values and a nil error.

### GetArrayMembers:

Accepts context and a key for performing redis `SMembers` operation.<br>
Upon success the result is returned as a []`string` and a nil error is returned.<br>
If the record does not exist returns empty response.

### GetStringArrayMembers:

Accepts context and a key for performing redis `SMembers` operation.<br>
Upon success the result is returned as a []`string` and a nil error is returned.<br>
The return value is striped from the " " for every string value.
If the record does not exist returns empty response.

### Get:

Accepts context and a key for performing redis `Get` operation.<br>
If the given key does not exist, a `ClassNotFound` error is returned. <br>
Upon success the result is returned as a `string` and a nil error is returned. 

### Incr:

Accepts context, and a key, for performing redis `Incr` operation.<br>
If the key does not exist, it is set to 0 before performing the operation.<br>
Upon success return the value of key after the increment.

### IncrByFloat:

Accepts context, a key and a value (as a `float64`), for performing redis `IncrByFloat` operation.<br>
If the key does not exist, it is set to 0 before performing the operation.<br>
Upon success return the value of key after the increment.

### GetByPattern:

Accepts context and a pattern key for performing redis `KEYS` operation.<br>
If the given pattern key does not match any key, a `ClassNotFound` error is returned. <br>
Upon success the matching values is returned as a list of `string` and a nil error is returned.

### GetAllValues:

Accepts context and a list of keys for performing redis `MGet` operation.<br>
If a given key does not exist it's ignored. <br>
Upon success a list of values returned as a `[]string` and a nil error is returned. 

### WatchTransaction:

Accepts context, keys, arguments, transaction attempts and the transaction function<br>
for performing redis transaction using `Watch` operation.<br>
If the transaction fails, return error.

### DELETE

Accepts context and a list of keys for performing `DEL` operation.<br>
If a given key does not found it's ignored.<br>
Returns the number of keys that were removed.

### DeleteByPattern

Accepts context and a pattern for performing `SCAN` and `DEL` operation by iterating the scan result.<br>
If a given pattern does not match any key it's ignored.<br>
Returns the number of total keys found, and the number of keys that were actually removed.

### TearDown:

Disconnects the client from redis.

