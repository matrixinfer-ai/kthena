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

package connectors

// ConnectorType represents the type of KV connector
type ConnectorType string

const (
	ConnectorTypeHTTP    ConnectorType = "http"
	ConnectorTypeLMCache ConnectorType = "lmcache"
	ConnectorTypeNIXL    ConnectorType = "nixl"
)

// Factory creates KV connectors based on type
type Factory struct {
	connectors map[ConnectorType]KVConnector
}

// NewFactory creates a new connector factory
func NewFactory() *Factory {
	return &Factory{
		connectors: make(map[ConnectorType]KVConnector),
	}
}

// RegisterConnector registers a connector with the factory
func (f *Factory) RegisterConnector(connectorType ConnectorType, connector KVConnector) {
	f.connectors[connectorType] = connector
}

// GetConnector returns a connector by type
func (f *Factory) GetConnector(connectorType ConnectorType) KVConnector {
	connector, ok := f.connectors[connectorType]
	if !ok {
		return nil
	}
	return connector
}

// NewDefaultFactory returns a factory with all default connectors registered
func NewDefaultFactory() *Factory {
	factory := NewFactory()

	// Register default connectors
	factory.RegisterConnector(ConnectorTypeHTTP, NewHTTPConnector())
	factory.RegisterConnector(ConnectorTypeLMCache, NewLMCacheConnector())
	factory.RegisterConnector(ConnectorTypeNIXL, NewNIXLConnector())

	return factory
}
