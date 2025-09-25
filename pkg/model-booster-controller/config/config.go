/*
Copyright The Volcano Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"encoding/json"
	"fmt"
)

const (
	// DefaultDownloaderImage is the default image used for downloading models.
	DefaultDownloaderImage = "kthena/downloader:latest"
	// DefaultRuntimeImage is the default image used for running model inference.
	DefaultRuntimeImage      = "kthena/runtime:latest"
	DefaultKVEventsPublisher = "zmq"
	DefaultKVEventsTopic     = "kv-events"
	DefaultKVEventsEndpoint  = "tcp://*:5557"
)

type ParseConfig struct {
	downloaderImage string
	runtimeImage    string
}

var Config ParseConfig

func (p *ParseConfig) DownloaderImage() string {
	if len(p.downloaderImage) == 0 {
		return DefaultDownloaderImage
	}
	return p.downloaderImage
}

func (p *ParseConfig) SetDownloaderImage(image string) {
	p.downloaderImage = image
}

func (p *ParseConfig) RuntimeImage() string {
	if len(p.runtimeImage) == 0 {
		return DefaultRuntimeImage
	}
	return p.runtimeImage
}

func (p *ParseConfig) SetRuntimeImage(image string) {
	p.runtimeImage = image
}

type KVEventsConfig struct {
	EnableKVCacheEvents bool   `json:"enable_kv_cache_events"`
	Publisher           string `json:"publisher"`
	Topic               string `json:"topic"`
	Endpoint            string `json:"endpoint"`
}

func GetDefaultKVEventsConfig() string {
	config := KVEventsConfig{
		EnableKVCacheEvents: true,
		Publisher:           DefaultKVEventsPublisher,
		Topic:               DefaultKVEventsTopic,
		Endpoint:            DefaultKVEventsEndpoint,
	}

	data, err := json.Marshal(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal KVEventsConfig: %v", err))
	}

	return string(data)
}
