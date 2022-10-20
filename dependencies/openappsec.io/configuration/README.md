# Configuration

This package aims to provide a configuration tool supporting both local and remote configuration. <br>
This is done by exposing a configuration service backed by a local repository (implemented via [viper](https://github.com/spf13/viper)) and a remote repository (implemented via [etcd](https://etcd.io/)).<br>
The remote repository can be toggled on/off. <br>

In addition this service allows for registration of hooks over a configuration key, these hooks are run when a change in the key's value occurs. 

## Initlization:

### General:
To create an instance of the configuration service use the `NewConfigurationService` function. <br>
This function accepts, besides its context, an implementation of two interfaces `Repository` and `RemoteRepository`. 

<table>
<tr>
<th>
Repository
</th>
<th>
RemoteRepository
</th>
</tr>

<tr>
<td>
<pre>
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
</pre>
</td>
<td>
<pre>
type RemoteRepository interface {
	Connect(ctx context.Context, hosts []string, timeout time.Duration, dir string) error
	GetConfig(ctx context.Context) (map[string]interface{}, error)
	WatchConfig() (&lt;-chan map[string]string, &lt;-chan error)
	Set(ctx context.Context, key string, value interface{}) error
	HealthCheck(ctx context.Context) (string, error)
}
</pre>
</td>
</tr>
</table>

The `Repository` serves as the source of data for the configuration service, any queries search for the requested data within it. <br>
> <b>NOTE</b>: Data in the repository will be loaded from the hardcoded path `./configs/bootstrapConfig.yaml`, from either working directory or executable directory

The `RemoteRepository` serves as an additional source of configuration data, overriding any local configurations.<br>
This is especially useful when we'd like to set a common config for a collection of services (for example, different replicas of the same service).<br>
> <b>NOTE</b>: Upon initialization the contents of the remote config are pulled and loaded into the local repository, overriding any local values. Any future changes in the remote repository are also updated to the local repository, this is achieved by watching the remote values for any changes. <b>Writing to the configuration service also writes to the remote configuration repository</b>.

### Implementation of the Repository and RemoteRepository interfaces:

This package, alongside the configuration service, exposes implementations for the aforementioned interfaces. <br>
The first is a `viper` based implementation for the `Repository` interface, and the second is an `etcd` based implementation of the `RemoteRepository` interface. <br>
You may use these when initializing you configuration service.

For example:

```go
package main

import (
	"context"
	"time"
	
	"openappsec.io/configuration"
	"openappsec.io/configuration/etcd"
	"openappsec.io/configuration/viper"
)


func main() {
	repository := viper.NewViper()
	remoteRepository := etcd.NewEtcd()	
	confSvc, err := configuration.NewConfigurationService(context.Background(), repository, remoteRepository)
}

```


#### How to configure the remote Etcd repository:

The creation of a working remote etcd repository requires certain inputs to allow for its configuration. <br>
In the example above we pass an uninitialized etcd repository to the configuration service, this is because <b>the configuration service itself configures it</b>.

To enable and configure the remote repository, the local repository should have the following fields loaded:

| Field Name | Field Value |
|------------|-------------|
| etcd.hosts | CSV list of etc hosts |
| etcd.timeout | timeout for etcd operations | 
| etcd.dir | this signifies the key in etcd under which your relevant configuration sits<br>Usually this is the service name |

To achive this you can paste the following into your service `./configs/bootstapConfig.yaml` file (values are examples):

```yaml
etcd:
  hosts: "http://configuration-server-svc"
  timeout: "10s"
  dir: "IntelligenceDB"
```

## Usage:

### Getting:

A series of "getter" method exist to retrieve configuration:

| Method | Description |
|---------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `GetAll` | Returns the entire configuration in `map[string]interface{}` format. |
| `Get` | Accepts a `string` key and returns the value stored under said key in `interface{}` format. |
| `GetString` | Accepts a `string` key and returns the value stored under said key in `string` format.  In case the key is not set in the configuration, a 'errors.ClassNotFound' will be returned. In case the value which the configuration holds for the given key is not a string, an 'errors.ClassInternal' will be returned. |
| `GetBool` | Accepts a `string` key and returns the value stored under said key in `bool` format.  In case the key is not set in the configuration, a 'errors.ClassNotFound' will be returned. In case the value which the configuration holds for the given key is not a boolean, an 'errors.ClassInternal' will be returned. |
| `GetInt` | Accepts a `string` key and returns the value stored under said key in `int` format.  In case the key is not set in the configuration, a 'errors.ClassNotFound' will be returned. In case the value which the configuration holds for the given key is not an integer, an 'errors.ClassInternal' will be returned. |
| `GetDuration` | Accepts a `string` key and returns the value stored under said key in `time.Duration` format.  In case the key is not set in the configuration, a 'errors.ClassNotFound' will be returned. In case the value which the configuration holds for the given key is not a valid duration, an 'errors.ClassInternal' will be returned. |

### Setting:

Several methods pertaining to setting values in the configuration are avaialable as part of the configuration service:

| Method | Description |
|-----------|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------:|
| `IsSet` | Returns a boolean value, indicating whether the given key (`string`) exists in the current configuration. |
| `Set` | Accepts a key (`string`) and a value (`interface{}`) and sets them in the configuration, both remote and local.  Additionally it invokes any hooks registered under the given key. |
| `SetMany` | Accepts a `map[string]interface{}` and calls the `Set` function for each key/value pair. |

### Hooks:

In addition to setting and getting configuration values this service also exposes the ability to register hooks for a given key in the configuration. When a change in the value stored in said key occurs, the hook will be invoked. <br>

To add a hook, use the `RegisterHook` method, passing it a key for which youd like to add a hook (`string`) and the hook itself (`func(value interface{}) error`).

For example, adding a hook to the configuration key `log.level`, when said key changes we would like to update our logging library to change its logging level:

```go
const (
	logLevelConfKey = "log.level"
)

func (a *App) setLogLevelFromConfiguration() error {
	logLevel, err := a.conf.GetString(logLevelConfKey)
	if err != nil {
		return err
	}

	err = log.SetLevel(logLevel)
	if err != nil {
		return errors.Wrapf(err, "Failed to set log level (%s)", logLevel).SetClass(errors.ClassBadInput)
	}

	log.WithContext(context.Background()).Infof("Set log level to: %s", logLevel)

	return nil
}

func (a *App) registerHook() error {
	a.conf.RegisterHook(logLevelConfKey, func(value interface{}) error {
		if err := a.setLogLevelFromConfiguration(); err != nil {
			return errors.Wrap(err, "Failed to set log level from configuration")
		}

		return nil
	})

	return nil
}
```

### HealthCheck:

If a remote configuration service is configured then this healthcheck function will invoke the remotes healthcheck and return the appropriate response. Otherwise this healthcehck always succeeds (seeing as the local repository cannot fail).

