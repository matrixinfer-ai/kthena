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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"k8s.io/klog/v2"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/conf"
)

// For the time being, the JWT is extracted directly from the fixed header name, and the configurable items are added later.
const (
	header = "Authorization"
	prefix = "Bearer "
)

func extractTokenFromHeader(req *http.Request) string {
	value := req.Header.Get(header)
	return strings.TrimPrefix(value, prefix)
}

type JWTValidator struct {
	enable bool
	cache  datastore.Store
}

// NewJWTValidator creates a new JWTValidator
func NewJWTValidator(store datastore.Store, gatewayConfig *conf.GatewayConfiguration) *JWTValidator {
	defaultValidator := &JWTValidator{
		enable: false,
	}
	if gatewayConfig != nil && gatewayConfig.Auth.JwksUri != "" {
		store.RotateJwks(gatewayConfig.Auth)
		defaultValidator.enable = true
		defaultValidator.cache = store
	}

	return defaultValidator
}

// parseAndValidateToken validates the token
func (j *JWTValidator) parseAndValidateToken(tokenStr string) error {
	key := j.cache.GetJwks()
	var token jwt.Token
	token, err := jwt.Parse([]byte(tokenStr), jwt.WithKeySet(key.Jwks, jws.WithInferAlgorithmFromKey(true)))
	if err != nil {
		return fmt.Errorf("failed to parse jwt: %w", err)
	}

	// Validate the claims in the token
	if err := j.validateClaims(token); err != nil {
		return fmt.Errorf("failed to validate claims: %w", err)
	}

	return nil
}

func (j *JWTValidator) validateClaims(token jwt.Token) error {
	if err := j.validateIssuer(token); err != nil {
		return fmt.Errorf("issuer validation failed: %w", err)
	}

	if err := j.validateAudiences(token); err != nil {
		return fmt.Errorf("audience validation failed: %w", err)
	}

	if err := j.validateTimeClaims(token); err != nil {
		return fmt.Errorf("time claims validation failed: %w", err)
	}

	return nil
}

func (j *JWTValidator) validateIssuer(token jwt.Token) error {
	var iss string
	jwks := j.cache.GetJwks()
	if err := token.Get("iss", &iss); err != nil || iss != jwks.Issuer {
		return fmt.Errorf("invalid issuer: expected %s, got %v", jwks.Issuer, iss)
	}
	return nil
}

func (j *JWTValidator) validateAudiences(token jwt.Token) error {
	var aud interface{}
	audinecesCache := j.cache.GetJwks().Audiences
	if len(audinecesCache) == 0 {
		// If audiences are not configured, we skip audience validation
		return nil
	}

	err := token.Get("aud", &aud)
	if err != nil {
		return fmt.Errorf("audience claim missing")
	}

	if aud == nil {
		return fmt.Errorf("need an audience")
	}

	// validate audience
	audMatched := false
	switch audVal := aud.(type) {
	case string:
		for _, expectedAud := range audinecesCache {
			if audVal == expectedAud {
				audMatched = true
				break
			}
		}
	case []string:
		for _, audItem := range audVal {
			for _, expectedAud := range audinecesCache {
				if audItem == expectedAud {
					audMatched = true
					break
				}
			}
		}
	}

	if !audMatched {
		return fmt.Errorf("audience mismatch: expected one of %v, got %v", audinecesCache, aud)
	}
	return nil
}

// validateTimeClaims validates the fields related to effective time (exp, nbf, iat)
// If token doesn't have exp, the token is invalid.
// For nbf and iat it's a little more lenient.
// If token doesn't have nbf, we assume it's valid.
// As same for iat.
func (j *JWTValidator) validateTimeClaims(token jwt.Token) error {
	now := time.Now()
	var exp, nbf, iat interface{}
	// Validate Token expiration(exp)
	if err := token.Get("exp", &exp); err == nil {
		if err := j.validateExpiration(exp, now); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("expiration claim (exp) missing")
	}

	// validate Token not before (nbf)
	if err := token.Get("nbf", &nbf); err == nil {
		if err := j.validateNotBefore(nbf, now); err != nil {
			return err
		}
	}

	// validate Token issued at (iat)
	if err := token.Get("iat", &iat); err == nil {
		if err := j.validateIssuedAt(iat, now); err != nil {
			return err
		}
	}

	return nil
}

func (j *JWTValidator) validateExpiration(exp interface{}, now time.Time) error {
	switch expVal := exp.(type) {
	case time.Time:
		if now.After(expVal) {
			return fmt.Errorf("token has expired")
		}
	case float64:
		expTime := time.Unix(int64(expVal), 0)
		if now.After(expTime) {
			return fmt.Errorf("token has expired")
		}
	case json.Number:
		if expInt, err := expVal.Int64(); err == nil {
			expTime := time.Unix(expInt, 0)
			if now.After(expTime) {
				return fmt.Errorf("token has expired")
			}
		} else {
			return fmt.Errorf("invalid exp value: %v", expVal)
		}
	default:
		return fmt.Errorf("unsupported exp type: %T", expVal)
	}
	return nil
}

func (j *JWTValidator) validateNotBefore(nbf interface{}, now time.Time) error {
	switch nbfVal := nbf.(type) {
	case time.Time:
		if now.Before(nbfVal) {
			return fmt.Errorf("token not yet valid")
		}
	case float64:
		nbfTime := time.Unix(int64(nbfVal), 0)
		if now.Before(nbfTime) {
			return fmt.Errorf("token not yet valid")
		}
	case json.Number:
		if nbfInt, err := nbfVal.Int64(); err == nil {
			nbfTime := time.Unix(nbfInt, 0)
			if now.Before(nbfTime) {
				return fmt.Errorf("token not yet valid")
			}
		} else {
			return fmt.Errorf("invalid nbf value: %v", nbfVal)
		}
	default:
		return fmt.Errorf("unsupported nbf type: %T", nbfVal)
	}
	return nil
}

func (j *JWTValidator) validateIssuedAt(iat interface{}, now time.Time) error {
	switch iatVal := iat.(type) {
	case time.Time:
		// iat should be before or equal to the current time
		// Allow a 1-minute clock skew
		if now.Add(1 * time.Minute).Before(iatVal) {
			return fmt.Errorf("token issued in the future")
		}
	case float64:
		iatTime := time.Unix(int64(iatVal), 0)
		if now.Add(1 * time.Minute).Before(iatTime) {
			return fmt.Errorf("token issued in the future")
		}
	case json.Number:
		if iatInt, err := iatVal.Int64(); err == nil {
			iatTime := time.Unix(iatInt, 0)
			if now.Add(1 * time.Minute).Before(iatTime) {
				return fmt.Errorf("token issued in the future")
			}
		} else {
			return fmt.Errorf("invalid iat value: %v", iatVal)
		}
	default:
		return fmt.Errorf("unsupported iat type: %T", iatVal)
	}
	return nil
}

// parseJWKS parse the jwks string
func (j *JWTValidator) parseJWKS(jwksStr string) (*jwk.Set, error) {
	keySet, err := jwk.Parse([]byte(jwksStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse inline JWKS: %w", err)
	}
	return &keySet, nil
}

func (j *JWTValidator) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if j.enable {
			// validate the token about the jwtRules
			token := extractTokenFromHeader(c.Request)
			klog.V(4).Infof("Extracted token: %s", token)
			err := j.parseAndValidateToken(token)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Unauthorized: %v", err)})
				return
			}
		}
		// TODO: add Authorization handler
		c.Next()
	}
}
