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

import (
	"fmt"
	"math/rand"

	"k8s.io/klog/v2"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

type TokenizerManagerConfig struct {
	EnableVLLMRemote bool
	EndpointTemplate string
	ModelServiceMap  map[string]string
}

type TokenizerManager struct {
	config TokenizerManagerConfig
}

func NewTokenizerManager(config TokenizerManagerConfig) *TokenizerManager {
	return &TokenizerManager{
		config: config,
	}
}

// GetTokenizer creates a tokenizer by randomly selecting from the provided pods
func (m *TokenizerManager) GetTokenizer(model string, pods []*datastore.PodInfo) Tokenizer {
	return m.createTokenizerFromPods(model, pods)
}

func (m *TokenizerManager) createTokenizerFromPods(model string, pods []*datastore.PodInfo) Tokenizer {
	if len(pods) == 0 {
		klog.Warningf("No pods provided for model %s", model)
		return nil
	}

	// Randomly select a pod to start with
	startIdx := rand.Intn(len(pods))

	// Try pods starting from random index, wrapping around if needed
	for i := 0; i < len(pods); i++ {
		podIdx := (startIdx + i) % len(pods)
		podInfo := pods[podIdx]

		endpoint := fmt.Sprintf(m.config.EndpointTemplate, podInfo.Pod.Status.PodIP)

		config := RemoteTokenizerConfig{
			Engine:             "vllm",
			Endpoint:           endpoint,
			Model:              model,
			AddSpecialTokens:   true,
			ReturnTokenStrings: false,
		}

		tok, err := NewRemoteTokenizer(config)
		if err != nil {
			klog.Warningf("Failed to create vLLM tokenizer for model %s at endpoint %s: %v", model, endpoint, err)
			continue
		}

		klog.V(4).Infof("TokenizerManager: successfully created tokenizer for model %s at endpoint %s", model, endpoint)
		return tok
	}

	klog.Warningf("Failed to create tokenizer for model %s after trying %d pods", model, len(pods))
	return nil
}
