/*
Copyright The Volcano Authors.

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

import (
	"encoding/json"
	"fmt"
)

const (
	vllmTokenizePath = "/tokenize"
)

type vllmAdapter struct {
	model string
}

func newVLLMAdapter(model string) *vllmAdapter {
	return &vllmAdapter{
		model: model,
	}
}

func (va *vllmAdapter) GetTokenizePath() string {
	return vllmTokenizePath
}

func (va *vllmAdapter) PrepareTokenizeRequest(input TokenizeInput) (interface{}, error) {
	switch input.Type {
	case CompletionInput:
		req := &vllmTokenizeCompletionRequest{
			Prompt:           input.Text,
			AddSpecialTokens: &input.AddSpecialTokens,
			ReturnTokenStrs:  &input.ReturnTokenStrings,
		}
		if va.model != "" {
			req.Model = va.model
		}
		return req, nil

	case ChatInput:
		req := &vllmTokenizeChatRequest{
			Messages:            input.Messages,
			AddSpecialTokens:    &input.AddSpecialTokens,
			AddGenerationPrompt: &input.AddGenerationPrompt,
			ReturnTokenStrs:     &input.ReturnTokenStrings,
		}
		if va.model != "" {
			req.Model = va.model
		}
		return req, nil

	default:
		return nil, fmt.Errorf("unsupported input type: %s", input.Type)
	}
}

func (va *vllmAdapter) ParseTokenizeResponse(data []byte) (*TokenizeResult, error) {
	var resp vllmTokenizeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse tokenize response: %w", err)
	}

	return &TokenizeResult{
		Count:        resp.Count,
		MaxModelLen:  resp.MaxModelLen,
		Tokens:       resp.Tokens,
		TokenStrings: resp.TokenStrs,
	}, nil
}
