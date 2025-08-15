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
	"context"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

type TokenizerPoolConfig struct {
	EnableVLLMRemote     bool
	EndpointTemplate     string
	HealthCheckPeriod    time.Duration
	TokenizerTTL         time.Duration
	MaxTokenizersPerPool int
	ModelServiceMap      map[string]string
	Timeout              time.Duration
}

type tokenizerEntry struct {
	tokenizer    Tokenizer
	endpoint     string
	lastUsed     time.Time
	lastHealthy  time.Time
	healthStatus bool
}

type tokenizerRef struct {
	model string
	tok   Tokenizer
}

type TokenizerPool struct {
	mu         sync.RWMutex
	tokenizers map[string]*tokenizerEntry
	config     TokenizerPoolConfig
	stopCh     chan struct{}
}

func NewTokenizerPool(config TokenizerPoolConfig) *TokenizerPool {
	pool := &TokenizerPool{
		tokenizers: make(map[string]*tokenizerEntry),
		config:     config,
		stopCh:     make(chan struct{}),
	}

	if config.EnableVLLMRemote && config.HealthCheckPeriod > 0 {
		pool.startHealthChecker()
	}

	return pool
}

func (p *TokenizerPool) GetTokenizer(model string, pods []*datastore.PodInfo) Tokenizer {
	p.mu.Lock()
	entry, exists := p.tokenizers[model]
	if exists && entry.healthStatus {
		entry.lastUsed = time.Now()
		tok := entry.tokenizer
		p.mu.Unlock()
		return tok
	}
	p.mu.Unlock()

	return p.createOrUpdateTokenizer(model, pods)
}

func (p *TokenizerPool) createOrUpdateTokenizer(model string, pods []*datastore.PodInfo) Tokenizer {
	p.mu.Lock()

	if entry, exists := p.tokenizers[model]; exists && entry.healthStatus {
		entry.lastUsed = time.Now()
		p.mu.Unlock()
		return entry.tokenizer
	}

	if len(p.tokenizers) >= p.config.MaxTokenizersPerPool {
		p.mu.Unlock()
		klog.Warningf("TokenizerPool reached max size %d", p.config.MaxTokenizersPerPool)
		return nil
	}

	endpoint := p.findVLLMEndpointForModel(model, pods)
	if endpoint == "" {
		p.mu.Unlock()
		klog.Warningf("No vLLM endpoint found for model %s", model)
		return nil
	}

	p.mu.Unlock()

	config := RemoteTokenizerConfig{
		Engine:             "vllm",
		Endpoint:           endpoint,
		Model:              model,
		Timeout:            p.config.Timeout,
		MaxRetries:         3,
		AddSpecialTokens:   true,
		ReturnTokenStrings: false,
	}

	tok, err := NewRemoteTokenizer(config)
	if err != nil {
		klog.Warningf("Failed to create vLLM tokenizer for model %s: %v", model, err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if remoteTok, ok := tok.(interface{ IsHealthy(context.Context) bool }); ok {
		if !remoteTok.IsHealthy(ctx) {
			klog.Warningf("Created tokenizer for model %s is not healthy", model)
			return nil
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, exists := p.tokenizers[model]; exists && entry.healthStatus {
		entry.lastUsed = time.Now()
		if closer, ok := tok.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return entry.tokenizer
	}

	now := time.Now()
	p.tokenizers[model] = &tokenizerEntry{
		tokenizer:    tok,
		endpoint:     endpoint,
		lastUsed:     now,
		lastHealthy:  now,
		healthStatus: true,
	}

	klog.V(3).Infof("Created vLLM tokenizer for model %s at endpoint %s", model, endpoint)

	return tok
}

func (p *TokenizerPool) findVLLMEndpointForModel(model string, pods []*datastore.PodInfo) string {
	if endpoint, exists := p.config.ModelServiceMap[model]; exists {
		return endpoint
	}

	klog.Infof("Looking for vLLM endpoint for model %s among %d pods", model, len(pods))

	for _, podInfo := range pods {
		pod := podInfo.Pod
		klog.Infof("Checking pod %s, ready: %v", pod.Name, isPodReady(pod))

		if !isPodReady(pod) {
			continue
		}

		modelName := GetModelNameFromPod(pod)
		klog.Infof("Pod %s model-name label: '%s', target model: '%s'", pod.Name, modelName, model)

		if modelName != model {
			continue
		}

		endpoint := fmt.Sprintf(p.config.EndpointTemplate, pod.Status.PodIP)
		klog.Infof("Found vLLM endpoint for model %s: %s", model, endpoint)
		return endpoint
	}

	klog.Infof("No vLLM endpoint found for model %s", model)
	return ""
}

func GetModelNameFromPod(pod *v1.Pod) string {
	if modelName := pod.Labels["registry.matrixinfer.ai/model-name"]; modelName != "" {
		return modelName
	}
	return ""
}

func isPodReady(pod *v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}

	return false
}

func (p *TokenizerPool) startHealthChecker() {
	ticker := time.NewTicker(p.config.HealthCheckPeriod)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.performHealthCheck()
				p.cleanupStaleTokenizers()
			case <-p.stopCh:
				return
			}
		}
	}()
}

func (p *TokenizerPool) performHealthCheck() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for model, entry := range p.tokenizers {
		if remoteTok, ok := entry.tokenizer.(interface{ IsHealthy(context.Context) bool }); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			healthy := remoteTok.IsHealthy(ctx)
			cancel()

			oldStatus := entry.healthStatus
			entry.healthStatus = healthy
			if healthy {
				entry.lastHealthy = time.Now()
			} else if oldStatus {
				klog.Warningf("Tokenizer for model %s is now unhealthy", model)
			}
		}
	}
}

func (p *TokenizerPool) cleanupStaleTokenizers() {
	var staleTokenizers []tokenizerRef

	p.mu.Lock()
	now := time.Now()
	for model, entry := range p.tokenizers {
		if now.Sub(entry.lastUsed) > p.config.TokenizerTTL {
			staleTokenizers = append(staleTokenizers, tokenizerRef{
				model: model,
				tok:   entry.tokenizer,
			})
			delete(p.tokenizers, model)
			klog.V(4).Infof("Removed stale tokenizer for model %s", model)
		}
	}
	p.mu.Unlock()

	for _, stale := range staleTokenizers {
		if closer, ok := stale.tok.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				klog.Errorf("Error closing tokenizer for model %s: %v", stale.model, err)
			}
		}
	}
}

func (p *TokenizerPool) Close() error {
	close(p.stopCh)

	var tokenizersToClose []tokenizerRef

	p.mu.Lock()
	for model, entry := range p.tokenizers {
		tokenizersToClose = append(tokenizersToClose, tokenizerRef{
			model: model,
			tok:   entry.tokenizer,
		})
	}
	p.tokenizers = make(map[string]*tokenizerEntry)
	p.mu.Unlock()

	for _, item := range tokenizersToClose {
		if closer, ok := item.tok.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				klog.Errorf("Error closing tokenizer for model %s: %v", item.model, err)
			}
		}
	}

	return nil
}
