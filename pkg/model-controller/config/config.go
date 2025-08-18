/*
Copyright MatrixInfer-AI Authors.

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

const (
	// DefaultModelInferDownloaderImage is the default image used for downloading models.
	DefaultModelInferDownloaderImage = "matrixinfer/downloader:latest"
	// DefaultModelInferRuntimeImage is the default image used for running model inference.
	DefaultModelInferRuntimeImage = "matrixinfer/runtime:latest"
)

type ParseConfig struct {
	modelInferDownloaderImage string
	modelInferRuntimeImage    string
}

var Config ParseConfig

func (p *ParseConfig) ModelInferDownloaderImage() string {
	if len(p.modelInferDownloaderImage) == 0 {
		return DefaultModelInferDownloaderImage
	}
	return p.modelInferDownloaderImage
}

func (p *ParseConfig) SetModelInferDownloaderImage(image string) {
	p.modelInferDownloaderImage = image
}

func (p *ParseConfig) ModelInferRuntimeImage() string {
	if len(p.modelInferRuntimeImage) == 0 {
		return DefaultModelInferRuntimeImage
	}
	return p.modelInferRuntimeImage
}

func (p *ParseConfig) SetModelInferRuntimeImage(image string) {
	p.modelInferRuntimeImage = image
}
