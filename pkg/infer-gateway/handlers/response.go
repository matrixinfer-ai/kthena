// Response handler can be used to handle inference response, do ratelimt for input tokens.
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"k8s.io/klog/v2"
)

// Define a struct to represent the OpenAI response body
type OpenAIResponseBody struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Text         string      `json:"text"`
		Index        int         `json:"index"`
		Logprobs     interface{} `json:"logprobs"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Function to parse the OpenAI response body
func ParseOpenAIResponseBody(resp *http.Response) (*OpenAIResponseBody, error) {
	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Unmarshal the JSON body into the struct
	var responseBody OpenAIResponseBody
	err = json.Unmarshal(body, &responseBody)
	if err != nil {
		return nil, err
	}

	return &responseBody, nil
}

// Do some model name conversion here
func (h *Handler) HandleResponse(r *http.Response) {
	// Parse the OpenAI response body
	responseBody, err := ParseOpenAIResponseBody(r)
	if err != nil {
		// Handle error
		return
	}

	// Do something with the response body
	_ = responseBody
}

const (
	streamingRespPrefix = "data: "
	streamingEndMsg     = "data: [DONE]"
)

// Referenced from https://github.com/kubernetes-sigs/gateway-api-inference-extension/blob/main/pkg/epp/handlers/response.go#L178-L265
func HandleNonStreamResponse(r *http.Response) error {
	// Read the response body
	resBody, err := io.ReadAll(r.Body)
	if err != nil {
		// Handle error
		return err
	}
	defer r.Body.Close()
	// Copy the body back to the request
	r.Body = io.NopCloser(bytes.NewBuffer(resBody))

	res := Response{}
	if err := json.Unmarshal(resBody, &res); err != nil {
		return fmt.Errorf("unmarshaling response body: %v", err)
	}

	klog.V(4).Infof("Response generated %v", res)
	return nil
}

func HandleStreamResponse(r *http.Response) {
	// Read the response body
	resBody, err := io.ReadAll(r.Body)
	if err != nil {
		// Handle error
		return
	}
	defer r.Body.Close()

	// Copy the body back to the request
	r.Body = io.NopCloser(bytes.NewBuffer(resBody))

	resText := string(resBody)
	if strings.Contains(resText, streamingEndMsg) {
		parsedResp := ParseRespForUsage(resText)
		// TODO: make use of the tokens to do rate limiting
		_ = parsedResp.Usage
	}
}

// Example message if "stream_options": {"include_usage": "true"} is included in the request:
// data: {"id":"...","object":"text_completion","created":1739400043,"model":"tweet-summary-0","choices":[],
// "usage":{"prompt_tokens":7,"total_tokens":17,"completion_tokens":10}}
//
// data: [DONE]
//
// Noticed that vLLM returns two entries in one response.
// We need to strip the `data:` prefix and next Data: [DONE] from the message to fetch response data.
//
// If include_usage is not included in the request, `data: [DONE]` is returned separately, which
// indicates end of streaming.
func ParseRespForUsage(
	responseText string,
) Response {
	response := Response{}

	lines := strings.Split(responseText, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, streamingRespPrefix) {
			continue
		}
		content := strings.TrimPrefix(line, streamingRespPrefix)
		if content == "[DONE]" {
			continue
		}

		byteSlice := []byte(content)
		if err := json.Unmarshal(byteSlice, &response); err != nil {
			klog.Error(err, "unmarshaling response body")
			continue
		}
	}

	return response
}

type Response struct {
	Usage Usage `json:"usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
