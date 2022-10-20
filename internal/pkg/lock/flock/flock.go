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

package flock

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"openappsec.io/errors"
	"openappsec.io/log"
)

const (
	confKeyFlockBase = "flock."
	confKeyFlockRoot = confKeyFlockBase + "root"
	confKeyFlockTTL  = confKeyFlockBase + "ttl"
)

// Configuration exposes an interface of configuration related actions
type Configuration interface {
	GetString(key string) (string, error)
	GetDuration(key string) (time.Duration, error)
	GetInt(key string) (int, error)
	GetBool(key string) (bool, error)
}

// Adapter for simple flock
type Adapter struct {
	root string
	ttl  time.Duration
}

// NewAdapter create a new flock adapter
func NewAdapter(conf Configuration) (*Adapter, error) {
	root, err := conf.GetString(confKeyFlockRoot)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load root directory")
	}
	ttl, err := conf.GetDuration(confKeyFlockTTL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load ttl")
	}

	a := &Adapter{
		root: root,
		ttl:  ttl,
	}

	err = os.MkdirAll(root, 0750)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	dir, err := ioutil.ReadDir(root)
	if err != nil {
		log.Warnf("fail to read dir: %v, err: %v", root, err)
		return a, err
	}
	for _, d := range dir {
		err = os.RemoveAll(path.Join([]string{root, d.Name()}...))
		if err != nil {
			log.Warnf("fail to remove dir contents: %v, err: %v", d.Name(), err)
		}
	}
	return a, nil
}

// Lock locks a key using a file
func (a *Adapter) Lock(ctx context.Context, key string) bool {
	key = strings.Replace(key, "/", "-", -1)
	f, err := os.Open(a.root + key)
	defer f.Close()
	if !os.IsNotExist(err) {
		return false
	}
	fh, err := os.Create(a.root + key)
	if err != nil {
		log.WithContext(ctx).Errorf("failed to acquire lock, err: %v", err)
		return false
	}
	defer fh.Close()
	go func() {
		time.Sleep(a.ttl)
		log.WithContext(ctx).Infof("release lock: %v", key)
		err := os.Remove(a.root + key)
		if err != nil {
			log.WithContext(ctx).Warnf("failed to delete lock, err: %v", err)
		}
	}()
	return true
}

// Unlock the lock of a given key
func (a *Adapter) Unlock(ctx context.Context, key string) error {
	key = strings.Replace(key, "/", "-", -1)
	log.WithContext(ctx).Infof("unlocking: %v", key)
	err := os.Remove(a.root + key)
	if err != nil {
		err = errors.Wrapf(err, "failed to unlock, key: %v", key)
	}
	return err
}

// TearDown clean up old locks
func (a *Adapter) TearDown(ctx context.Context) error {
	log.WithContext(ctx).Info("tear down")
	dir, err := ioutil.ReadDir(a.root)
	if err != nil {
		log.Warnf("fail to read dir: %v, err: %v", a.root, err)
		return err
	}
	for _, d := range dir {
		err = os.RemoveAll(path.Join([]string{a.root, d.Name()}...))
		if err != nil {
			log.Warnf("fail to remove dir contents: %v, err: %v", d.Name(), err)
		}
	}
	return nil
}
