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

// Request handler can be used to handle inference request, by reading the model name, and do ratelimt for input tokens.
package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// Import openai-go instead of defining our own struct
// OenAI have different kind of interfaces which use different kind of request and response body
type OpenAIRequestBody struct {
	Model string `json:"model"`
	_     interface{}
}

// Function to parse the OpenAI request body
func ParseOpenAIRequestBody(r *http.Request) (string, error) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	// Copy the body back to the request
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Unmarshal the JSON body into the struct
	var requestBody OpenAIRequestBody
	err = json.Unmarshal(body, &requestBody)
	if err != nil {
		return "", err
	}

	return requestBody.Model, nil
}

// Do some model name conversion here
func (h *Handler) HandleRequestBody(r *http.Request) {

}
