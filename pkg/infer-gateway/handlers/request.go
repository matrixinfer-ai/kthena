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

// Do some model name convertion here
func (h *Handler) HandleRequestBody(r *http.Request) {

}
