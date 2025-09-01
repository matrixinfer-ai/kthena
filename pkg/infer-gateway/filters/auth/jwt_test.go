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

package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/conf"
)

func TestNewJWTAuthenticator(t *testing.T) {
	tests := []struct {
		name          string
		config        *conf.GatewayConfiguration
		expectEnabled bool
		expectRotator bool
	}{
		{
			name:          "nil config",
			config:        nil,
			expectEnabled: false,
			expectRotator: false,
		},
		{
			name: "empty JWKS URI",
			config: &conf.GatewayConfiguration{
				Auth: conf.AuthenticationConfig{
					JwksUri: "",
				},
			},
			expectEnabled: false,
			expectRotator: false,
		},
		{
			name: "invalid JWKS URI",
			config: &conf.GatewayConfiguration{
				Auth: conf.AuthenticationConfig{
					JwksUri:   "invalid-url",
					Issuer:    "test-issuer",
					Audiences: []string{"test-audience"},
				},
			},
			expectEnabled: false,
			expectRotator: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewJWTAuthenticator(tt.config)
			assert.NotNil(t, validator)
			assert.Equal(t, tt.expectEnabled, validator.IsEnabled())

			if tt.expectRotator {
				assert.NotNil(t, validator.rotator)
			} else {
				assert.Nil(t, validator.rotator)
			}
		})
	}
}

func TestNewJwks(t *testing.T) {
	tests := []struct {
		name      string
		config    conf.AuthenticationConfig
		expectNil bool
	}{
		{
			name: "empty URI",
			config: conf.AuthenticationConfig{
				JwksUri: "",
			},
			expectNil: true,
		},
		{
			name: "invalid URI",
			config: conf.AuthenticationConfig{
				JwksUri: "invalid-url",
				Issuer:  "test-issuer",
			},
			expectNil: true,
		},
		{
			name: "valid URI",
			config: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Issuer:    "test-issuer",
				Audiences: []string{"test-audience"},
			},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwks := NewJwks(tt.config)

			if tt.expectNil {
				assert.Nil(t, jwks)
			} else {
				require.NotNil(t, jwks)
				assert.Equal(t, tt.config.Issuer, jwks.Issuer)
				assert.Equal(t, tt.config.Audiences, jwks.Audiences)
				assert.Equal(t, tt.config.JwksUri, jwks.Uri)
				assert.NotNil(t, jwks.Jwks)
				assert.Equal(t, defaultRefreshInterval, jwks.ExpiredTime)
			}
		})
	}
}
