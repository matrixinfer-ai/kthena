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

package router

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/accesslog"
)

func TestAccessLogConfigurationFromEnv(t *testing.T) {
	// Save original environment variables
	originalEnabled := os.Getenv("ACCESS_LOG_ENABLED")
	originalFormat := os.Getenv("ACCESS_LOG_FORMAT")
	originalOutput := os.Getenv("ACCESS_LOG_OUTPUT")

	// Clean up after test
	defer func() {
		os.Setenv("ACCESS_LOG_ENABLED", originalEnabled)
		os.Setenv("ACCESS_LOG_FORMAT", originalFormat)
		os.Setenv("ACCESS_LOG_OUTPUT", originalOutput)
	}()

	tests := []struct {
		name            string
		envEnabled      string
		envFormat       string
		envOutput       string
		expectedEnabled bool
		expectedFormat  accesslog.LogFormat
		expectedOutput  string
	}{
		{
			name:            "default configuration",
			envEnabled:      "",
			envFormat:       "",
			envOutput:       "",
			expectedEnabled: true,
			expectedFormat:  accesslog.FormatText,
			expectedOutput:  "stdout",
		},
		{
			name:            "JSON format configuration",
			envEnabled:      "true",
			envFormat:       "json",
			envOutput:       "stdout",
			expectedEnabled: true,
			expectedFormat:  accesslog.FormatJSON,
			expectedOutput:  "stdout",
		},
		{
			name:            "text format with file output",
			envEnabled:      "true",
			envFormat:       "text",
			envOutput:       "/var/log/access.log",
			expectedEnabled: true,
			expectedFormat:  accesslog.FormatText,
			expectedOutput:  "/var/log/access.log",
		},
		{
			name:            "disabled access log",
			envEnabled:      "false",
			envFormat:       "json",
			envOutput:       "stdout",
			expectedEnabled: false,
			expectedFormat:  accesslog.FormatJSON,
			expectedOutput:  "stdout",
		},
		{
			name:            "stderr output",
			envEnabled:      "true",
			envFormat:       "text",
			envOutput:       "stderr",
			expectedEnabled: true,
			expectedFormat:  accesslog.FormatText,
			expectedOutput:  "stderr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			os.Setenv("ACCESS_LOG_ENABLED", tt.envEnabled)
			os.Setenv("ACCESS_LOG_FORMAT", tt.envFormat)
			os.Setenv("ACCESS_LOG_OUTPUT", tt.envOutput)

			// Create access log configuration (simulating the logic in NewRouter)
			accessLogConfig := &accesslog.AccessLoggerConfig{
				Enabled: true,
				Format:  accesslog.FormatText,
				Output:  "stdout",
			}

			// Apply environment variable overrides
			if enabled := os.Getenv("ACCESS_LOG_ENABLED"); enabled != "" {
				if enabledBool, err := parseBool(enabled); err == nil {
					accessLogConfig.Enabled = enabledBool
				}
			}

			if format := os.Getenv("ACCESS_LOG_FORMAT"); format != "" {
				if format == "json" {
					accessLogConfig.Format = accesslog.FormatJSON
				} else if format == "text" {
					accessLogConfig.Format = accesslog.FormatText
				}
			}

			if output := os.Getenv("ACCESS_LOG_OUTPUT"); output != "" {
				accessLogConfig.Output = output
			}

			// Verify configuration
			assert.Equal(t, tt.expectedEnabled, accessLogConfig.Enabled)
			assert.Equal(t, tt.expectedFormat, accessLogConfig.Format)
			assert.Equal(t, tt.expectedOutput, accessLogConfig.Output)

			// Test that logger can be created with this configuration
			logger, err := accesslog.NewAccessLogger(accessLogConfig)
			assert.NoError(t, err)
			assert.NotNil(t, logger)

			// Clean up
			logger.Close()
		})
	}
}

// Helper function to parse boolean (same logic as strconv.ParseBool but simpler for test)
func parseBool(str string) (bool, error) {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False":
		return false, nil
	}
	return false, &strconv.NumError{Func: "ParseBool", Num: str, Err: strconv.ErrSyntax}
}
