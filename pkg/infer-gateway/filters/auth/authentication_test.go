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
	"os"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/conf"
)

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
			}
		})
	}
}

func TestJWTValidatorValidateIssuer(t *testing.T) {
	tokenWithValidIssuer, err := jwt.NewBuilder().
		Issuer("valid-issuer").
		Build()
	assert.NoError(t, err, "Failed to build token with valid issuer")

	tokenWithInvalidIssuer, err := jwt.NewBuilder().
		Issuer("invalid-issuer").
		Build()
	assert.NoError(t, err, "Failed to build token with invalid issuer")

	tokenWithoutIssuer, err := jwt.NewBuilder().
		Subject("test-subject").
		Build()
	assert.NoError(t, err, "Failed to build token without issuer")

	tokenWithEmptyIssuer, err := jwt.NewBuilder().
		Issuer("").
		Build()
	assert.NoError(t, err, "Failed to build token with empty issuer")

	tests := []struct {
		name           string
		token          jwt.Token
		expectedIssuer string
		auth           conf.AuthenticationConfig
		expectError    bool
		errorMsg       string
	}{
		{
			name:           "Valid issuer match",
			token:          tokenWithValidIssuer,
			expectedIssuer: "valid-issuer",
			auth: conf.AuthenticationConfig{
				Issuer:  "valid-issuer",
				JwksUri: "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
			},
			expectError: false,
		},
		{
			name:           "Invalid issuer - mismatch",
			token:          tokenWithInvalidIssuer,
			expectedIssuer: "valid-issuer",
			auth: conf.AuthenticationConfig{
				Issuer:  "valid-issuer",
				JwksUri: "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
			},
			expectError: true,
			errorMsg:    "invalid issuer: expected valid-issuer, got invalid-issuer",
		},
		{
			name:           "Invalid issuer - missing in token",
			token:          tokenWithoutIssuer,
			expectedIssuer: "valid-issuer",
			auth: conf.AuthenticationConfig{
				Issuer:  "valid-issuer",
				JwksUri: "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
			},
			expectError: true,
			errorMsg:    "invalid issuer: expected valid-issuer, got ",
		},
		{
			name:           "Invalid issuer - empty in token",
			token:          tokenWithEmptyIssuer,
			expectedIssuer: "valid-issuer",
			auth: conf.AuthenticationConfig{
				Issuer:  "valid-issuer",
				JwksUri: "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
			},
			expectError: true,
			errorMsg:    "invalid issuer: expected valid-issuer, got ",
		},
		{
			name:           "Empty expected issuer with valid token issuer",
			token:          tokenWithValidIssuer,
			expectedIssuer: "",
			auth: conf.AuthenticationConfig{
				Issuer:  "",
				JwksUri: "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
			},
			expectError: true,
			errorMsg:    "invalid issuer: expected , got valid-issuer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := datastore.New()
			jwtValidator := &JWTValidator{
				cache: store,
			}
			gatewayConf := &conf.GatewayConfiguration{
				Auth: tt.auth,
			}
			wrtieAuthConfigToFile(t, "auth_config.yaml", gatewayConf)
			defer os.Remove("auth_config.yaml")

			jwtValidator.cache.FlushJwks("auth_config.yaml")
			err := jwtValidator.validateIssuer(tt.token)

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

	tokenWithEmptyAudience, err := jwt.NewBuilder().
		Audience([]string{""}).
		Build()
	assert.NoError(t, err, "Failed to build token with empty audience")

	tests := []struct {
		name              string
		token             jwt.Token
		expectedAudiences []string
		auth              conf.AuthenticationConfig
		expectError       bool
		errorMsg          string
	}{
		{
			name:              "Valid single audience match",
			token:             tokenWithSingleAudience,
			expectedAudiences: []string{"valid-audience"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"valid-audience"},
			},
			expectError: false,
		},
		{
			name:              "Valid single audience match (string type)",
			token:             tokenWithSingleAudienceAsInterface,
			expectedAudiences: []string{"valid-audience"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"valid-audience"},
			},
			expectError: false,
		},
		{
			name:              "Valid multiple audiences match (first)",
			token:             tokenWithMultipleAudiences,
			expectedAudiences: []string{"audience1", "audience2"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"audience1", "audience2"},
			},
			expectError: false,
		},
		{
			name:              "Valid multiple audiences match (second)",
			token:             tokenWithMultipleAudiences,
			expectedAudiences: []string{"audience3"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"audience3"},
			},
			expectError: false,
		},
		{
			name:              "Invalid audience - no match",
			token:             tokenWithSingleAudience,
			expectedAudiences: []string{"different-audience"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"different-audience"},
			},
			expectError: true,
			errorMsg:    "audience mismatch: expected one of [different-audience], got [valid-audience]",
		},
		{
			name:              "Invalid audience - missing in token",
			token:             tokenWithoutAudience,
			expectedAudiences: []string{"valid-audience"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"valid-audience"},
			},
			expectError: true,
			errorMsg:    "audience claim missing",
		},
		{
			name:              "Empty expected audiences",
			token:             tokenWithSingleAudience,
			expectedAudiences: []string{},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{},
			},
			expectError: true,
			errorMsg:    "audience mismatch: expected one of [], got [valid-audience]",
		},
		{
			name:              "Empty audience in token",
			token:             tokenWithEmptyAudience,
			expectedAudiences: []string{""},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{""},
			},
			expectError: false,
		},
		{
			name:              "Multiple expected audiences with match",
			token:             tokenWithSingleAudience,
			expectedAudiences: []string{"other-audience", "valid-audience", "another-audience"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"other-audience", "valid-audience", "another-audience"},
			},
			expectError: false,
		},
		{
			name:              "Token with empty audience array",
			token:             tokenWithEmptyAudience,
			expectedAudiences: []string{"expected-audience"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"expected-audience"},
			},
			expectError: true,
			errorMsg:    "audience mismatch: expected one of [expected-audience], got []",
		},
		{
			name: "Special characters in audience",
			token: func() jwt.Token {
				token, _ := jwt.NewBuilder().
					Audience([]string{"https://api.example.com", "api://internal"}).
					Build()
				return token
			}(),
			expectedAudiences: []string{"https://api.example.com"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"https://api.example.com"},
			},
			expectError: false,
		},
		{
			name: "Case sensitive audience comparison",
			token: func() jwt.Token {
				token, _ := jwt.NewBuilder().
					Audience([]string{"Test-Audience"}).
					Build()
				return token
			}(),
			expectedAudiences: []string{"test-audience"},
			auth: conf.AuthenticationConfig{
				JwksUri:   "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json",
				Audiences: []string{"test-audience"},
			},
			expectError: true,
			errorMsg:    "audience mismatch: expected one of [test-audience], got [Test-Audience]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := datastore.New()
			jwtValidator := &JWTValidator{
				cache: store,
			}
			gatewayConf := &conf.GatewayConfiguration{
				Auth: tt.auth,
			}
			wrtieAuthConfigToFile(t, "auth_config.yaml", gatewayConf)
			defer os.Remove("auth_config.yaml")

			jwtValidator.cache.FlushJwks("auth_config.yaml")
			err := jwtValidator.validateAudiences(tt.token)

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

func TestJWTValidatorParseAndValidateTokenRealIstioData(t *testing.T) {
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

func wrtieAuthConfigToFile(t *testing.T, filePath string, config *conf.GatewayConfiguration) {
	data, err := yaml.Marshal(config)
	assert.NoError(t, err, "Failed to marshal gatewayConfiguration")
	err = os.WriteFile(filePath, data, 0644)
	assert.NoError(t, err, "Failed to write gatewayConfiguration to file")
}
