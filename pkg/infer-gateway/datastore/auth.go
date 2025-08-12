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
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
)

type Jwks struct {
	Jwks          jwk.Set
	ExpiredTime   time.Duration
	LastFreshTime time.Time
}

func (s *store) GetJwks(jwksURI string) *Jwks {
	if cached, ok := s.jwksCache.Load(jwksURI); ok {
		if keySet, ok := cached.(*Jwks); ok {
			return keySet
		}
	}

	return nil
}

func (s *store) FetchJwks(rule aiv1alpha1.JWTRule) error {
	if rule.JwksURI == "" {
		return nil
	}

	keySet, err := jwk.Fetch(context.Background(), rule.JwksURI)
	fmt.Printf("jwks is %#v", keySet)
	if err != nil {
		return err
	}

	if rule.JwksExpiredTime == nil {
		s.jwksCache.Store(rule.JwksURI, &Jwks{
			Jwks:          keySet,
			ExpiredTime:   time.Hour * 24 * 7,
			LastFreshTime: time.Now(),
		})
	} else {
		s.jwksCache.Store(rule.JwksURI, &Jwks{
			Jwks:          keySet,
			ExpiredTime:   rule.JwksExpiredTime.Duration,
			LastFreshTime: time.Now(),
		})
	}

	s.jwksCache.Range(func(key, value any) bool {
		fmt.Printf("key: %s, value: %v", key, value)
		if _, ok := value.(Jwks); ok {
			fmt.Printf("\n=========== success ===========\n")
			return true
		}
		return false
	})

	return nil
}

func (s *store) DeleteJwks(ms *aiv1alpha1.ModelServer) {
	if len(ms.Spec.JWTRules) == 0 {
		return
	}

	for _, rule := range ms.Spec.JWTRules {
		if rule.JwksURI != "" {
			s.jwksCache.Delete(rule.JwksURI)
		}
	}
}

func (s *store) updateJwks(oldMs, newMs *aiv1alpha1.ModelServer) {
	if oldMs == nil {
		if len(newMs.Spec.JWTRules) == 0 {
			return
		}

		for _, rule := range newMs.Spec.JWTRules {
			if rule.JwksURI != "" {
				go s.FetchJwks(rule)
			}
		}
		return
	}

	if len(oldMs.Spec.JWTRules) != 0 {
		for _, oldRule := range oldMs.Spec.JWTRules {
			if oldRule.JwksURI != "" {
				s.jwksCache.Delete(oldRule.JwksURI)
			}
		}
	}

	if len(newMs.Spec.JWTRules) != 0 {
		for _, newRule := range newMs.Spec.JWTRules {
			if newRule.JwksURI != "" {
				go s.FetchJwks(newRule)
			}
		}
	}
}
