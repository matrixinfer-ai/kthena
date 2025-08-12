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
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/stretchr/testify/assert"

	networkingv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
)

func TestExtractTokenFromHeader(t *testing.T) {
	tests := []struct {
		name          string
		headers       map[string]string
		jwtRule       networkingv1alpha1.JWTRule
		expectedToken string
		expectError   bool
	}{
		{
			name: "Token found without prefix",
			headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			},
			jwtRule: networkingv1alpha1.JWTRule{
				FromHeader: networkingv1alpha1.JWTHeader{
					Name: "Authorization",
				},
			},
			expectedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expectError:   false,
		},
		{
			name: "Token found with prefix",
			headers: map[string]string{
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			},
			jwtRule: networkingv1alpha1.JWTRule{
				FromHeader: networkingv1alpha1.JWTHeader{
					Name:   "Authorization",
					Prefix: "Bearer ",
				},
			},
			expectedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expectError:   false,
		},
		{
			name: "Token not found - missing header",
			headers: map[string]string{
				"X-Custom": "value",
			},
			jwtRule: networkingv1alpha1.JWTRule{
				FromHeader: networkingv1alpha1.JWTHeader{
					Name: "Authorization",
				},
			},
			expectedToken: "",
			expectError:   true,
		},
		{
			name: "Token not found - wrong prefix",
			headers: map[string]string{
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			},
			jwtRule: networkingv1alpha1.JWTRule{
				FromHeader: networkingv1alpha1.JWTHeader{
					Name:   "Authorization",
					Prefix: "Token ",
				},
			},
			expectedToken: "",
			expectError:   true,
		},
		{
			name: "Empty header value",
			headers: map[string]string{
				"Authorization": "",
			},
			jwtRule: networkingv1alpha1.JWTRule{
				FromHeader: networkingv1alpha1.JWTHeader{
					Name: "Authorization",
				},
			},
			expectedToken: "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			token, err := extractTokenFromHeader(req, tt.jwtRule)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedToken, token)
			}
		})
	}
}

func TestExtractTokenFromParam(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		jwtRule       networkingv1alpha1.JWTRule
		expectedToken string
		expectError   bool
	}{
		{
			name: "Token found in query parameter",
			url:  "/test?access_token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			jwtRule: networkingv1alpha1.JWTRule{
				FromParam: "access_token",
			},
			expectedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expectError:   false,
		},
		{
			name: "Token found in different parameter name",
			url:  "/test?jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			jwtRule: networkingv1alpha1.JWTRule{
				FromParam: "jwt",
			},
			expectedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expectError:   false,
		},
		{
			name: "Token not found - missing parameter",
			url:  "/test?other_param=value",
			jwtRule: networkingv1alpha1.JWTRule{
				FromParam: "access_token",
			},
			expectedToken: "",
			expectError:   true,
		},
		{
			name: "Token not found - empty parameter value",
			url:  "/test?access_token=",
			jwtRule: networkingv1alpha1.JWTRule{
				FromParam: "access_token",
			},
			expectedToken: "",
			expectError:   true,
		},
		{
			name: "Multiple parameters with correct one present",
			url:  "/test?other=value&access_token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c&another=test",
			jwtRule: networkingv1alpha1.JWTRule{
				FromParam: "access_token",
			},
			expectedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expectError:   false,
		},
		{
			name: "Parameter not present in URL with query string",
			url:  "/test?param1=value1&param2=value2",
			jwtRule: networkingv1alpha1.JWTRule{
				FromParam: "access_token",
			},
			expectedToken: "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			token, err := extractTokenFromParam(req, tt.jwtRule)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedToken, token)
			}
		})
	}
}

func TestJWTValidatorParseJWKS(t *testing.T) {
	validJWKS := `{
		"keys": [
			{
				"kty": "RSA",
				"use": "sig",
				"kid": "example-key-id",
				"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-cs2JgrXV7g3bMsRBrKZt2SX12gE454h4cN67cp28r94S4Kz399CI0OAA5E9rs3oc7_nJyb-2ICmD7ES6l7EHcR43ZUlM94zLvFAP1Qh-u3Cd1jl2565OUOzP87gVXz4Wv8d7w4VLE9pxmgc5F29akpXfrXyfs-U8TzG1e13CJ2KwBPND9Q7kwDoZ9F2d8pu78FFc0GmmkswkN5kqdJrO8Ig2c91cXgyf21z4UZQk9Y36018H4XCzvJq4b1bGNC2KkWQTmW5QclD9dyvN63wVO09HWhd7NGrlN63vDpfAp6XmmCy66XxjP9aFntB6xVC0f8yj8B57sXzYBIU0-3",
				"e": "AQAB"
			}
		]
	}`

	invalidJWKS := `{
		"keys": [
			{
				"kty": "RSA",
				"use": "sig",
				"invalid_field": 
			}
		]
	}`

	emptyJWKS := `{"keys": []}`

	nonJSON := `not a json string`

	tests := []struct {
		name        string
		jwksStr     string
		expectError bool
	}{
		{
			name:        "Valid JWKS with RSA key",
			jwksStr:     validJWKS,
			expectError: false,
		},
		{
			name:        "Empty JWKS",
			jwksStr:     emptyJWKS,
			expectError: false,
		},
		{
			name:        "Invalid JWKS format",
			jwksStr:     invalidJWKS,
			expectError: true,
		},
		{
			name:        "Non-JSON string",
			jwksStr:     nonJSON,
			expectError: true,
		},
		{
			name:        "Empty string",
			jwksStr:     "",
			expectError: true,
		},
	}

	jwtValidator := &JWTValidator{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keySet, err := jwtValidator.parseJWKS(tt.jwksStr)
			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				assert.Nil(t, keySet, "Expected nil keySet when error occurs")
			} else {
				assert.NoError(t, err, "Expected no error")
				assert.NotNil(t, keySet, "Expected non-nil keySet")

				// validate return type
				_, ok := (*keySet).(jwk.Set)
				assert.True(t, ok, "Expected result to be of type *jwk.Set")
			}
		})
	}
}

func TestJWTValidatorValidateIssuer(t *testing.T) {
	tokenWithIssuer, err := jwt.NewBuilder().
		Issuer("valid-issuer").
		Build()
	assert.NoError(t, err, "Failed to build token with issuer")

	tokenWithoutIssuer, err := jwt.NewBuilder().
		Subject("test-subject").
		Build()
	assert.NoError(t, err, "Failed to build token without issuer")

	ruleWithIssuer := networkingv1alpha1.JWTRule{
		Issuer: "valid-issuer",
	}

	ruleWithDifferentIssuer := networkingv1alpha1.JWTRule{
		Issuer: "different-issuer",
	}

	ruleWithoutIssuer := networkingv1alpha1.JWTRule{
		Issuer: "",
	}

	tests := []struct {
		name        string
		token       jwt.Token
		rule        networkingv1alpha1.JWTRule
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid issuer match",
			token:       tokenWithIssuer,
			rule:        ruleWithIssuer,
			expectError: false,
		},
		{
			name:        "Invalid issuer - mismatch",
			token:       tokenWithIssuer,
			rule:        ruleWithDifferentIssuer,
			expectError: true,
			errorMsg:    "invalid issuer: expected different-issuer, got valid-issuer",
		},
		{
			name:        "Invalid issuer - missing in token",
			token:       tokenWithoutIssuer,
			rule:        ruleWithIssuer,
			expectError: true,
			errorMsg:    "invalid issuer: expected valid-issuer, got ",
		},
		{
			name:        "No issuer validation required",
			token:       tokenWithIssuer,
			rule:        ruleWithoutIssuer,
			expectError: false,
		},
		{
			name:        "No issuer validation required and missing in token",
			token:       tokenWithoutIssuer,
			rule:        ruleWithoutIssuer,
			expectError: false,
		},
	}
	jwtValidator := &JWTValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jwtValidator.validateIssuer(tt.token, tt.rule)
			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if tt.errorMsg != "" {
					assert.EqualError(t, err, tt.errorMsg, "Error message should match expected")
				}
			} else {
				assert.NoError(t, err, "Expected no error")
			}
		})
	}
}

func TestJWTValidatorValidateAudiences(t *testing.T) {
	tokenWithSingleAudience, err := jwt.NewBuilder().
		Audience([]string{"valid-audience"}).
		Build()
	assert.NoError(t, err, "Failed to build token with single audience")

	tokenWithMultipleAudiences, err := jwt.NewBuilder().
		Claim("aud", []string{"audience1", "audience2", "audience3"}).
		Build()
	assert.NoError(t, err, "Failed to build token with multiple audiences")

	tokenWithSingleAudienceAsInterface, err := jwt.NewBuilder().
		Claim("aud", "valid-audience").
		Build()
	assert.NoError(t, err, "Failed to build token with single audience as interface")

	tokenWithoutAudience, err := jwt.NewBuilder().
		Subject("test-subject").
		Build()
	assert.NoError(t, err, "Failed to build token without audience")

	ruleWithSingleAudience := networkingv1alpha1.JWTRule{
		Audiences: []string{"valid-audience"},
	}

	ruleWithMultipleAudiences := networkingv1alpha1.JWTRule{
		Audiences: []string{"audience1", "audience2"},
	}

	ruleWithNonMatchingAudience := networkingv1alpha1.JWTRule{
		Audiences: []string{"different-audience"},
	}

	ruleWithoutAudiences := networkingv1alpha1.JWTRule{
		Audiences: []string{},
	}

	tests := []struct {
		name        string
		token       jwt.Token
		rule        networkingv1alpha1.JWTRule
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid single audience match",
			token:       tokenWithSingleAudience,
			rule:        ruleWithSingleAudience,
			expectError: false,
		},
		{
			name:        "Valid single audience match (string type)",
			token:       tokenWithSingleAudienceAsInterface,
			rule:        ruleWithSingleAudience,
			expectError: false,
		},
		{
			name:        "Valid multiple audiences match (first)",
			token:       tokenWithMultipleAudiences,
			rule:        ruleWithMultipleAudiences,
			expectError: false,
		},
		{
			name:        "Valid multiple audiences match (second)",
			token:       tokenWithMultipleAudiences,
			rule:        networkingv1alpha1.JWTRule{Audiences: []string{"audience3"}},
			expectError: false,
		},
		{
			name:        "Invalid audience - no match",
			token:       tokenWithSingleAudience,
			rule:        ruleWithNonMatchingAudience,
			expectError: true,
			errorMsg:    "audience mismatch: expected one of [different-audience], got [valid-audience]",
		},
		{
			name:        "Invalid audience - missing in token",
			token:       tokenWithoutAudience,
			rule:        ruleWithSingleAudience,
			expectError: true,
			errorMsg:    "audience claim missing",
		},
		{
			name:        "No audience validation required",
			token:       tokenWithSingleAudience,
			rule:        ruleWithoutAudiences,
			expectError: false,
		},
		{
			name:        "No audience validation required and missing in token",
			token:       tokenWithoutAudience,
			rule:        ruleWithoutAudiences,
			expectError: false,
		},
	}
	jwtValidator := &JWTValidator{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jwtValidator.validateAudiences(tt.token, tt.rule)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJWTValidatorValidateExpiration(t *testing.T) {
	jwtValidator := &JWTValidator{}

	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	tests := []struct {
		name        string
		exp         interface{}
		now         time.Time
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid expiration - time.Time in future",
			exp:         futureTime,
			now:         now,
			expectError: false,
		},
		{
			name:        "Invalid expiration - time.Time in past",
			exp:         pastTime,
			now:         now,
			expectError: true,
			errorMsg:    "token has expired",
		},
		{
			name:        "Valid expiration - time.Time equals now",
			exp:         now,
			now:         now,
			expectError: false,
		},
		{
			name:        "Valid expiration - float64 in future",
			exp:         float64(futureTime.Unix()),
			now:         now,
			expectError: false,
		},
		{
			name:        "Invalid expiration - float64 in past",
			exp:         float64(pastTime.Unix()),
			now:         now,
			expectError: true,
			errorMsg:    "token has expired",
		},
		{
			name:        "Valid expiration - float64 equals now",
			exp:         float64(now.Unix()),
			now:         now,
			expectError: true,
			errorMsg:    "token has expired",
		},
		{
			name:        "Invalid expiration - json.Number with invalid format",
			exp:         json.Number("invalid"),
			now:         now,
			expectError: true,
			errorMsg:    "invalid exp value: invalid",
		},
		{
			name:        "Unsupported exp type - string",
			exp:         "not-a-time",
			now:         now,
			expectError: true,
			errorMsg:    "unsupported exp type: string",
		},
		{
			name:        "Unsupported exp type - bool",
			exp:         true,
			now:         now,
			expectError: true,
			errorMsg:    "unsupported exp type: bool",
		},
		{
			name:        "Unsupported exp type - int",
			exp:         12345,
			now:         now,
			expectError: true,
			errorMsg:    "unsupported exp type: int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jwtValidator.validateExpiration(tt.exp, tt.now)
			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if tt.errorMsg != "" {
					assert.EqualError(t, err, tt.errorMsg, "Error message should match expected")
				}
			} else {
				assert.NoError(t, err, "Expected no error")
			}
		})
	}
}

func TestJWTValidatorValidateNotBefore(t *testing.T) {
	jwtValidator := &JWTValidator{}

	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	tests := []struct {
		name        string
		nbf         interface{}
		now         time.Time
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid nbf - time.Time in past",
			nbf:         pastTime,
			now:         now,
			expectError: false,
		},
		{
			name:        "Invalid nbf - time.Time in future",
			nbf:         futureTime,
			now:         now,
			expectError: true,
			errorMsg:    "token not yet valid",
		},
		{
			name:        "Valid nbf - time.Time equals now",
			nbf:         now,
			now:         now,
			expectError: false,
		},
		{
			name:        "Valid nbf - float64 in past",
			nbf:         float64(pastTime.Unix()),
			now:         now,
			expectError: false,
		},
		{
			name:        "Invalid nbf - float64 in future",
			nbf:         float64(futureTime.Unix()),
			now:         now,
			expectError: true,
			errorMsg:    "token not yet valid",
		},
		{
			name:        "Valid nbf - float64 equals now",
			nbf:         float64(now.Unix()),
			now:         now,
			expectError: false,
		},
		// edge clase
		{
			name:        "Boundary - nbf just before now",
			nbf:         now.Add(-1 * time.Nanosecond),
			now:         now,
			expectError: false,
		},
		{
			name:        "Boundary - nbf just after now",
			nbf:         now.Add(1 * time.Nanosecond),
			now:         now,
			expectError: true,
			errorMsg:    "token not yet valid",
		},
		{
			name:        "Large timestamp values",
			nbf:         float64(2147483647),
			now:         now,
			expectError: true,
			errorMsg:    "token not yet valid",
		},
		{
			name:        "Negative timestamp values",
			nbf:         float64(-1000),
			now:         now,
			expectError: false,
		},
		{
			name:        "JSON number with decimal",
			nbf:         json.Number("1234567890.5"),
			now:         now.Add(1 * time.Hour),
			expectError: true,
			errorMsg:    "invalid nbf value: 1234567890.5",
		},
		{
			name:        "Zero values",
			nbf:         time.Unix(0, 0),
			now:         now,
			expectError: false,
		},
		{
			name:        "Unsupported nbf type - string",
			nbf:         "not-a-time",
			now:         now,
			expectError: true,
			errorMsg:    "unsupported nbf type: string",
		},
		{
			name:        "Unsupported nbf type - bool",
			nbf:         true,
			now:         now,
			expectError: true,
			errorMsg:    "unsupported nbf type: bool",
		},
		{
			name:        "Unsupported nbf type - int",
			nbf:         12345,
			now:         now,
			expectError: true,
			errorMsg:    "unsupported nbf type: int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jwtValidator.validateNotBefore(tt.nbf, tt.now)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if tt.errorMsg != "" {
					assert.EqualError(t, err, tt.errorMsg, "Error message should match expected")
				}
			} else {
				assert.NoError(t, err, "Expected no error")
			}
		})
	}
}

func TestJWTValidatorValidateIssuedAt(t *testing.T) {
	jwtValidator := &JWTValidator{}

	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)
	futureTimeWithSkew := now.Add(30 * time.Second)

	tests := []struct {
		name        string
		iat         interface{}
		now         time.Time
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid iat - time.Time in past",
			iat:         pastTime,
			now:         now,
			expectError: false,
		},
		{
			name:        "Valid iat - time.Time equals now",
			iat:         now,
			now:         now,
			expectError: false,
		},
		{
			name:        "Invalid iat - time.Time in future beyond skew",
			iat:         futureTime,
			now:         now,
			expectError: true,
			errorMsg:    "token issued in the future",
		},
		{
			name:        "Valid iat - time.Time in future within skew",
			iat:         futureTimeWithSkew,
			now:         now,
			expectError: false,
		},
		{
			name:        "Valid iat - float64 in past",
			iat:         float64(pastTime.Unix()),
			now:         now,
			expectError: false,
		},
		{
			name:        "Valid iat - float64 equals now",
			iat:         float64(now.Unix()),
			now:         now,
			expectError: false,
		},
		{
			name:        "Invalid iat - float64 in future beyond skew",
			iat:         float64(futureTime.Unix()),
			now:         now,
			expectError: true,
			errorMsg:    "token issued in the future",
		},
		{
			name:        "Valid iat - float64 in future within skew",
			iat:         float64(futureTimeWithSkew.Unix()),
			now:         now,
			expectError: false,
		},
		// Edge Class
		{
			name:        "Boundary - iat just before now minus skew",
			iat:         now.Add(-59 * time.Second),
			now:         now,
			expectError: false,
		},
		{
			name:        "Boundary - iat just after now plus skew",
			iat:         now.Add(61 * time.Second),
			now:         now,
			expectError: true,
			errorMsg:    "token issued in the future",
		},
		{
			name:        "Large timestamp values",
			iat:         float64(2147483647),
			now:         now,
			expectError: true,
			errorMsg:    "token issued in the future",
		},
		{
			name:        "Negative timestamp values",
			iat:         float64(-1000),
			now:         now,
			expectError: false,
		},
		{
			name:        "JSON number with decimal",
			iat:         json.Number("1234567890.5"),
			now:         now.Add(-2 * time.Hour),
			expectError: true,
			errorMsg:    "invalid iat value: 1234567890.5",
		},
		{
			name:        "Zero values",
			iat:         time.Unix(0, 0),
			now:         now,
			expectError: false,
		},
		{
			name:        "Unsupported iat type - string",
			iat:         "not-a-time",
			now:         now,
			expectError: true,
			errorMsg:    "unsupported iat type: string",
		},
		{
			name:        "Unsupported iat type - bool",
			iat:         true,
			now:         now,
			expectError: true,
			errorMsg:    "unsupported iat type: bool",
		},
		{
			name:        "Unsupported iat type - int",
			iat:         12345,
			now:         now,
			expectError: true,
			errorMsg:    "unsupported iat type: int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jwtValidator.validateIssuedAt(tt.iat, tt.now)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if tt.errorMsg != "" {
					assert.EqualError(t, err, tt.errorMsg, "Error message should match expected")
				}
			} else {
				assert.NoError(t, err, "Expected no error")
			}
		})
	}
}

func TestJWTValidator_ParseAndValidateToken_RealIstioData(t *testing.T) {

	jwtToken := "eyJhbGciOiJSUzI1NiIsImtpZCI6IkRIRmJwb0lVcXJZOHQyenBBMnFYZkNtcjVWTzVaRXI0UnpIVV8tZW52dlEiLCJ0eXAiOiJKV1QifQ.eyJleHAiOjQ2ODU5ODk3MDAsImZvbyI6ImJhciIsImlhdCI6MTUzMjM4OTcwMCwiaXNzIjoidGVzdGluZ0BzZWN1cmUuaXN0aW8uaW8iLCJzdWIiOiJ0ZXN0aW5nQHNlY3VyZS5pc3Rpby5pbyJ9.CfNnxWP2tcnR9q0vxyxweaF3ovQYHYZl82hAUsn21bwQd9zP7c-LS9qd_vpdLG4Tn1A15NxfCjp5f7QNBUo-KC9PJqYpgGbaXhaGx7bEdFWjcwv3nZzvc7M__ZpaCERdwU7igUmJqYGBYQ51vr2njU9ZimyKkfDe3axcyiBZde7G6dabliUosJvvKOPcKIWPccCgefSj_GNfwIip3-SsFdlR7BtbVUcqR-yv-XOxJ3Uc1MI0tz3uMiiZcyPV7sNCU4KRnemRIMHVOfuvHsU60_GhGbiSFzgPTAa9WTltbnarTbxudb_YEOx12JiwYToeX0DCPb43W1tzIBxgm8NxUg"

	jwks := `{
		"keys":[{
			"e":"AQAB",
			"kid":"DHFbpoIUqrY8t2zpA2qXfCmr5VO5ZEr4RzHU_-envvQ",
			"kty":"RSA",
			"n":"xAE7eB6qugXyCAG3yhh7pkDkT65pHymX-P7KfIupjf59vsdo91bSP9C8H07pSAGQO1MV_xFj9VswgsCg4R6otmg5PV2He95lZdHtOcU5DXIg_pbhLdKXbi66GlVeK6ABZOUW3WYtnNHD-91gVuoeJT_DwtGGcp4ignkgXfkiEm4sw-4sfb4qdt5oLbyVpmW6x9cfa7vs2WTfURiCrBoUqgBo_-4WTiULmmHSGZHOjzwa8WtrtOQGsAFjIbno85jp6MnGGGZPYZbDAa_b3y5u-YpW7ypZrvD8BgtKVjgtQgZhLAGezMt0ua3DRrWnKqTZ0BJ_EyxOGuHJrLsn00fnMQ"
		}]
	}`
	keySet, err := jwk.Parse([]byte(jwks))
	assert.NoError(t, err, "Failed to parse JWKS")

	_, err = jwt.Parse([]byte(jwtToken), jwt.WithKeySet(keySet, jws.WithInferAlgorithmFromKey(true)))
	assert.NoError(t, err, "Failed to parse JWT")
}

func TestJwTParse(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err, "Failed to generate RSA private key")

	realKey, err := jwk.Import(privateKey)
	assert.NoError(t, err, "Failed to import RSA private key as JWK")
	realKey.Set(jwk.KeyIDKey, "test-key-id")
	realKey.Set(jwk.AlgorithmKey, "RS256")

	// create another key for testing
	otherKey, err := jwk.Import([]byte("test-key"))
	assert.NoError(t, err, "Failed to import other key")
	otherKey.Set(jwk.AlgorithmKey, jwa.NoSignature())
	otherKey.Set(jwk.KeyIDKey, "other-key-id")

	keySet := jwk.NewSet()
	keySet.AddKey(realKey)
	keySet.AddKey(otherKey)
	v, err := jwk.PublicSetOf(keySet)
	assert.NoError(t, err, "Failed to get public set of keys")
	keySet = v

	token := jwt.New()
	token.Set("iss", "test-issuer")
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256(), realKey))
	assert.NoError(t, err, "Failed to sign token")

	_, err = jwt.Parse(signed, jwt.WithKeySet(keySet))
	assert.NoError(t, err, "Failed to parse signed token")
}
