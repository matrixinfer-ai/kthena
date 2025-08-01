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
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"

	networkingv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

func extractTokenFromHeader(req *http.Request, rule networkingv1alpha1.JWTRule) (string, error) {
	for _, header := range rule.FromHeader {
		value := req.Header.Get(header.Name)
		if value != "" {
			if header.Prefix != "" {
				if strings.HasPrefix(value, header.Prefix) {
					return strings.TrimPrefix(value, header.Prefix), nil
				}
			} else {
				return value, nil
			}
		}
	}
	return "", fmt.Errorf("jwt not found in headers")
}

func extractTokenFromParam(req *http.Request, rule networkingv1alpha1.JWTRule) (string, error) {
	for _, param := range rule.FromParams {
		if value := req.URL.Query().Get(param); value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("jwt not found in params")
}

type JWTValidator struct {
	cache datastore.Store
}

// NewJWTValidator creates a new JWTValidator
func NewJWTValidator(store datastore.Store) *JWTValidator {
	return &JWTValidator{
		cache: store,
	}
}

// ValidateRequest validates a request against a set of JWT rules
func (j *JWTValidator) ValidateRequest(req *http.Request, rules []networkingv1alpha1.JWTRule) (jwt.Token, networkingv1alpha1.JWTRule, error) {
	// preprocess request
	potentialRules := j.filterRulesByRequest(req, rules)

	// iterate through potential rules
	for _, rule := range potentialRules {
		token, err := j.validateWithRule(req, rule)
		if err == nil && token != nil {
			return token, rule, nil
		}

		continue
	}

	return nil, networkingv1alpha1.JWTRule{}, fmt.Errorf("unable to validate jwt with any rule")
}

// filterRulesByRequest filters rules by request
func (j *JWTValidator) filterRulesByRequest(req *http.Request, rules []networkingv1alpha1.JWTRule) []networkingv1alpha1.JWTRule {
	var potential []networkingv1alpha1.JWTRule

	for _, rule := range rules {
		if j.hasPotentialInHeader(req, rule) {
			potential = append(potential, rule)
			continue
		}

		if j.hasPotentialInParams(req, rule) {
			potential = append(potential, rule)
		}
	}

	return potential
}

// hasPotentialInHeader checks if the JWT rule has potential in the request header
func (j *JWTValidator) hasPotentialInHeader(req *http.Request, rule networkingv1alpha1.JWTRule) bool {
	for _, header := range rule.FromHeader {
		if req.Header.Get(header.Name) != "" {
			return true
		}
	}
	return false
}

// hasPotentialInParams checks if the JWT rule has potential in the request parameters
func (j *JWTValidator) hasPotentialInParams(req *http.Request, rule networkingv1alpha1.JWTRule) bool {
	for _, param := range rule.FromParams {
		if req.URL.Query().Get(param) != "" {
			return true
		}
	}
	return false
}

// validateWithRule validates the request with the given JWT rule
func (j *JWTValidator) validateWithRule(req *http.Request, rule networkingv1alpha1.JWTRule) (jwt.Token, error) {
	if len(rule.FromHeader) > 0 {
		if tokenStr, err := extractTokenFromHeader(req, rule); err == nil {
			return j.parseAndValidateToken(tokenStr, rule)
		}
	}

	if len(rule.FromParams) > 0 {
		if tokenStr, err := extractTokenFromParam(req, rule); err == nil {
			return j.parseAndValidateToken(tokenStr, rule)
		}
	}

	return nil, fmt.Errorf("unable to extract jwt")
}

// parseAndValidateToken validates the token
func (j *JWTValidator) parseAndValidateToken(tokenStr string, rule networkingv1alpha1.JWTRule) (jwt.Token, error) {
	// 获取验证密钥
	key, err := j.getVerificationKey(rule)
	if err != nil {
		return nil, err
	}

	token, err := jwt.Parse([]byte(tokenStr), jwt.WithKeySet(*key))
	if err != nil {
		return nil, fmt.Errorf("failed to parse jwt: %w", err)
	}

	if err := j.validateIssuer(token, rule); err != nil {
		return nil, err
	}

	if err := j.validateAudiences(token, rule); err != nil {
		return nil, err
	}

	if err := j.validateTimeClaims(token); err != nil {
		return nil, err
	}

	return token, nil
}

// getVerificationKey gets the verification key for the given JWTRule
func (j *JWTValidator) getVerificationKey(rule networkingv1alpha1.JWTRule) (*jwk.Set, error) {
	if rule.JwksURI != "" {
		key, err := j.getJWKSKey(rule.JwksURI)
		if err != nil {
			return nil, fmt.Errorf("failed to get key from JWKS: %w", err)
		}
		return key, nil
	} else if rule.Jwks != "" {
		key, err := j.parseJWKS(rule.Jwks)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JWKS: %w", err)
		}
		return key, nil
	}

	return nil, fmt.Errorf("no key source specified")
}

func (j *JWTValidator) validateIssuer(token jwt.Token, rule networkingv1alpha1.JWTRule) error {
	if rule.Issuer != "" {
		if iss, ok := token.Get("iss"); !ok || iss != rule.Issuer {
			return fmt.Errorf("invalid issuer: expected %s, got %v", rule.Issuer, iss)
		}
	}
	return nil
}

func (j *JWTValidator) validateAudiences(token jwt.Token, rule networkingv1alpha1.JWTRule) error {
	if len(rule.Audiences) > 0 {
		aud, ok := token.Get("aud")
		if !ok {
			return fmt.Errorf("audience claim missing")
		}

		// validate audience
		audMatched := false
		switch audVal := aud.(type) {
		case string:
			for _, expectedAud := range rule.Audiences {
				if audVal == expectedAud {
					audMatched = true
					break
				}
			}
		case []interface{}:
			for _, audItem := range audVal {
				if audStr, ok := audItem.(string); ok {
					for _, expectedAud := range rule.Audiences {
						if audStr == expectedAud {
							audMatched = true
							break
						}
					}
				}
			}
		}

		if !audMatched {
			return fmt.Errorf("audience mismatch: expected one of %v, got %v", rule.Audiences, aud)
		}
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

	// Validate Token expiration(exp)
	if exp, ok := token.Get("exp"); ok {
		if err := j.validateExpiration(exp, now); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("expiration claim (exp) missing")
	}

	// validate Token not before (nbf)
	if nbf, ok := token.Get("nbf"); ok {
		if err := j.validateNotBefore(nbf, now); err != nil {
			return err
		}
	}

	// validate Token issued at (iat)
	if iat, ok := token.Get("iat"); ok {
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

// getJWKSKey get the jwks key from cache
func (j *JWTValidator) getJWKSKey(jwksURI string) (*jwk.Set, error) {
	keySet, err := j.cache.GetAndUpdateJWKS(jwksURI)
	if err != nil {
		return nil, err
	}
	return keySet, nil
}

// parseJWKS parse the jwks string
func (j *JWTValidator) parseJWKS(jwksStr string) (*jwk.Set, error) {
	keySet, err := jwk.Parse([]byte(jwksStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse inline JWKS: %w", err)
	}
	return &keySet, nil
}

func (j *JWTValidator) Authenticate(jwtRules []networkingv1alpha1.JWTRule) gin.HandlerFunc {
	return func(c *gin.Context) {
		// The current modelServer does not have a JWTRule and therefore does not perform JWT authentication
		if len(jwtRules) == 0 {
			c.Next()
			return
		}

		// validate the token about the jwtRules
		_, _, err := j.ValidateRequest(c.Request, jwtRules)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Unauthorized: %v", err)})
			return
		}

		c.Next()
	}
}
