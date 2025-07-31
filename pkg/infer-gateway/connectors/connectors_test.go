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

import (
	"testing"
	"time"

	"matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
)

func TestConnectorFactory(t *testing.T) {
	factory := GetDefaultFactory()

	// Test HTTP connector creation
	httpConnector, err := factory.CreateConnector(ConnectorTypeHTTP)
	if err != nil {
		t.Fatalf("Failed to create HTTP connector: %v", err)
	}
	if httpConnector.Name() != "http" {
		t.Errorf("Expected HTTP connector name 'http', got '%s'", httpConnector.Name())
	}

	// Test LMCache connector creation
	lmcacheConnector, err := factory.CreateConnector(ConnectorTypeLMCache)
	if err != nil {
		t.Fatalf("Failed to create LMCache connector: %v", err)
	}
	if lmcacheConnector.Name() != "lmcache" {
		t.Errorf("Expected LMCache connector name 'lmcache', got '%s'", lmcacheConnector.Name())
	}

	// Test NIXL connector creation
	nixlConnector, err := factory.CreateConnector(ConnectorTypeNIXL)
	if err != nil {
		t.Fatalf("Failed to create NIXL connector: %v", err)
	}
	if nixlConnector.Name() != "nixl" {
		t.Errorf("Expected NIXL connector name 'nixl', got '%s'", nixlConnector.Name())
	}

	// Test unknown connector type
	_, err = factory.CreateConnector("unknown")
	if err == nil {
		t.Error("Expected error for unknown connector type, got nil")
	}
}

func TestHTTPConnector(t *testing.T) {
	connector := NewHTTPConnector()

	if connector.Name() != "http" {
		t.Errorf("Expected HTTP connector name 'http', got '%s'", connector.Name())
	}
}

func TestLMCacheConnector(t *testing.T) {
	connector := NewLMCacheConnector()

	if connector.Name() != "lmcache" {
		t.Errorf("Expected LMCache connector name 'lmcache', got '%s'", connector.Name())
	}
}

func TestNIXLConnector(t *testing.T) {
	connector := NewNIXLConnector()

	if connector.Name() != "nixl" {
		t.Errorf("Expected NIXL connector name 'nixl', got '%s'", connector.Name())
	}
}

func TestConnectorFactoryWithConfig(t *testing.T) {
	factory := GetDefaultFactory()

	// Test creating connector with config
	prefillTimeout := "45s"
	decodeTimeout := "180s"
	maxAttempts := int32(5)
	backoffBase := "2s"

	spec := &v1alpha1.KVConnectorSpec{
		Type: "http",
		Timeouts: &v1alpha1.KVConnectorTimeouts{
			Prefill: &prefillTimeout,
			Decode:  &decodeTimeout,
		},
		Retry: &v1alpha1.KVConnectorRetry{
			MaxAttempts: &maxAttempts,
			BackoffBase: &backoffBase,
		},
	}

	config := ParseConnectorConfig(ConnectorTypeHTTP, spec)
	if config.PrefillTimeout != 45*time.Second {
		t.Errorf("Expected prefill timeout 45s, got %v", config.PrefillTimeout)
	}
	if config.DecodeTimeout != 180*time.Second {
		t.Errorf("Expected decode timeout 180s, got %v", config.DecodeTimeout)
	}
	if config.MaxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", config.MaxRetries)
	}
	if config.BackoffBase != 2*time.Second {
		t.Errorf("Expected backoff base 2s, got %v", config.BackoffBase)
	}

	// Test creating connector with config
	connector, err := factory.CreateConnectorWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create connector with config: %v", err)
	}
	if connector.Name() != "http" {
		t.Errorf("Expected HTTP connector name 'http', got '%s'", connector.Name())
	}
}

func TestParseConnectorConfig(t *testing.T) {
	// Test with nil spec (default values)
	config := ParseConnectorConfig(ConnectorTypeHTTP, nil)
	if config.Type != ConnectorTypeHTTP {
		t.Errorf("Expected connector type %s, got %s", ConnectorTypeHTTP, config.Type)
	}
	if config.PrefillTimeout != 30*time.Second {
		t.Errorf("Expected default prefill timeout 30s, got %v", config.PrefillTimeout)
	}
	if config.DecodeTimeout != 120*time.Second {
		t.Errorf("Expected default decode timeout 120s, got %v", config.DecodeTimeout)
	}
	if config.MaxRetries != 3 {
		t.Errorf("Expected default max retries 3, got %d", config.MaxRetries)
	}
	if config.BackoffBase != 1*time.Second {
		t.Errorf("Expected default backoff base 1s, got %v", config.BackoffBase)
	}

	// Test with empty spec (default values)
	emptySpec := &v1alpha1.KVConnectorSpec{Type: "http"}
	config = ParseConnectorConfig(ConnectorTypeHTTP, emptySpec)
	// Should still use defaults
	if config.PrefillTimeout != 30*time.Second {
		t.Errorf("Expected default prefill timeout 30s, got %v", config.PrefillTimeout)
	}
}
