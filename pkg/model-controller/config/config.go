package config

const (
	DefaultModelInferDownloaderImage = "matrixinfer/downloader:latest"
	DefaultModelInferRuntimeImage    = "matrixinfer/runtime:latest"
)

type ParseConfig struct {
	modelInferDownloaderImage string
	modelInferRuntimeImage    string
}

var Config ParseConfig

func (p *ParseConfig) GetModelInferDownloaderImage() string {
	if len(p.modelInferDownloaderImage) == 0 {
		return DefaultModelInferDownloaderImage
	}
	return p.modelInferDownloaderImage
}

func (p *ParseConfig) SetModelInferDownloaderImage(image string) {
	p.modelInferDownloaderImage = image
}

func (p *ParseConfig) GetModelInferRuntimeImage() string {
	if len(p.modelInferRuntimeImage) == 0 {
		return DefaultModelInferRuntimeImage
	}
	return p.modelInferRuntimeImage
}

func (p *ParseConfig) SetModelInferRuntimeImage(image string) {
	p.modelInferRuntimeImage = image
}
