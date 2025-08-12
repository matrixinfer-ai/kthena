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
	"testing"

	"github.com/stretchr/testify/assert"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
)

func TestStoreFetchJwksRealURL(t *testing.T) {
	store := New()

	jwtRule := aiv1alpha1.JWTRule{
		JwksURI: "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
	}

	err := store.FetchJwks(jwtRule)
	assert.NoError(t, err)
	cachedJwks := store.GetJwks(jwtRule.JwksURI)
	assert.NotNil(t, cachedJwks.Jwks, "JWKS should be stored in cache")
	assert.NotZero(t, cachedJwks.LastFreshTime, "LastFreshTime should be set")
}
