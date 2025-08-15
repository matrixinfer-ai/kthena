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

package datastore

import (
	"context"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"k8s.io/klog/v2"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/conf"
)

type Jwks struct {
	Jwks          jwk.Set
	Audiences     []string
	Issuer        string
	ExpiredTime   time.Duration
	LastFreshTime time.Time
}

func NewJwks(configMapPath string) *Jwks {
	gatewayConfig, err := conf.ParseGatewayConfig(configMapPath)
	if err != nil {
		klog.Fatalf("failed to parse gateway config: %v", err)
		return nil
	}
	if gatewayConfig.Auth.JwksUri == "" {
		klog.V(4).Info("JWKS URI is empty, skipping JWKS initialization")
		return nil
	}
	keySet, err := jwk.Fetch(context.Background(), gatewayConfig.Auth.JwksUri)
	if err != nil {
		klog.Errorf("failed to fetch JWKS from %s: %v", gatewayConfig.Auth.JwksUri, err)
		return nil
	}
	return &Jwks{
		Jwks:          keySet,
		Audiences:     gatewayConfig.Auth.Audiences,
		Issuer:        gatewayConfig.Auth.Issuer,
		ExpiredTime:   time.Hour * 24 * 7, // Default to 7 days
		LastFreshTime: time.Now(),
	}
}

func (s *store) GetJwks() *Jwks {
	return s.jwksCache
}

func (s *store) FlushJwks(configMapPath string) {
	// This function is used to refresh the JWKS cache.
	// It can be called periodically or when a specific event occurs.
	newJwks := NewJwks(configMapPath)
	if newJwks != nil {
		s.jwksCache = newJwks
		klog.V(4).Infof("JWKS cache refreshed successfully")
	} else {
		klog.Error("Failed to refresh JWKS cache")
	}
}
