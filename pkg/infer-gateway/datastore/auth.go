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

const (
	maxRetry = 3
)

type Jwks struct {
	Jwks      jwk.Set
	Audiences []string
	Issuer    string
	// Used to update jwks
	Uri         string
	ExpiredTime time.Duration
}

func NewJwks(config conf.AuthenticationConfig) *Jwks {
	if config.JwksUri == "" {
		klog.V(4).Info("JWKS URI is empty, skipping JWKS initialization")
		return nil
	}

	var keySet jwk.Set
	var err error
	for i := 0; i < maxRetry; i++ {
		keySet, err = jwk.Fetch(context.Background(), config.JwksUri)
		if err != nil {
			klog.V(4).Infof("failed to fetch JWKS from %s: %v", config.JwksUri, err)
		} else {
			return &Jwks{
				Jwks:      keySet,
				Audiences: config.Audiences,
				Issuer:    config.Issuer,
				Uri:       config.JwksUri,
				// Default expiration time is set to 7 days
				ExpiredTime: time.Hour * 24 * 7, // Default to 7 days
			}
		}
	}

	return nil
}

func (s *store) GetJwks() Jwks {
	s.routeMutex.RLock()
	defer s.routeMutex.RUnlock()
	// The jwksCache has already been initialized in the NewStore, so there will be no null pointers
	return *s.jwksCache
}

func (s *store) RotateJwks(config conf.AuthenticationConfig) {
	// This function is used to refresh the JWKS cache.
	// It can be called periodically or when a specific event occurs.
	newJwks := NewJwks(config)
	if newJwks != nil {
		s.routeMutex.Lock()
		s.jwksCache = newJwks
		s.routeMutex.Unlock()
		klog.V(4).Infof("JWKS cache refreshed successfully")
	} else {
		klog.Error("Failed to refresh JWKS cache")
	}
}

func (s *store) jwksRefresher(ctx context.Context) {
	var interval time.Duration
	var config conf.AuthenticationConfig
	jwks := s.GetJwks()
	if jwks.Uri == "" {
		// If jwks.uri is empty, means not configuration of authentication.
		// No need to fresh jwks.
		return
	} else {
		interval = jwks.ExpiredTime
		config.Audiences = jwks.Audiences
		config.Issuer = jwks.Issuer
		config.JwksUri = jwks.Uri
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case <-ctx.Done():
			return
		default:
			s.RotateJwks(config)
			klog.V(4).Info("JWKS refreshed successfully")
		}
	}
}
