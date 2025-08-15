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

package tokenization

import "fmt"

type ErrInvalidConfig struct {
	Message string
}

func (e ErrInvalidConfig) Error() string {
	return fmt.Sprintf("invalid config: %s", e.Message)
}

type ErrTokenizationFailed struct {
	Message string
	Cause   error
}

func (e ErrTokenizationFailed) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("tokenization failed: %s: %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("tokenization failed: %s", e.Message)
}

type ErrHTTPRequest struct {
	StatusCode int
	Message    string
	Cause      error
}

func (e ErrHTTPRequest) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("HTTP %d: %s: %v", e.StatusCode, e.Message, e.Cause)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}
