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

package utils

import (
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/common"
)

var (
	GPUCacheUsage     = "gpu_usage"
	RequestWaitingNum = "request_waiting_num"
	RequestRunningNum = "request_running_num"
	TPOT              = "TPOT"
	TTFT              = "TTFT"
)

func GetNamespaceName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func ParsePrompt(body map[string]interface{}) (common.ChatMessage, error) {
	if prompt, ok := body["prompt"]; ok {
		promptStr, ok := prompt.(string)
		if !ok {
			return common.ChatMessage{}, fmt.Errorf("prompt is not a string")
		}
		return common.ChatMessage{
			Text: promptStr,
		}, nil
	}

	if messages, ok := body["messages"]; ok {
		messageList, ok := messages.([]interface{})
		if !ok {
			return common.ChatMessage{}, fmt.Errorf("messages is not a list")
		}

		var msgs []common.Message
		for _, message := range messageList {
			msgMap, ok := message.(map[string]interface{})
			if !ok {
				continue
			}

			role, ok := msgMap["role"].(string)
			if !ok {
				continue
			}

			content, ok := msgMap["content"].(string)
			if !ok {
				continue
			}

			msgs = append(msgs, common.Message{
				Role:    role,
				Content: content,
			})
		}

		return common.ChatMessage{
			Messages: msgs,
		}, nil
	}

	return common.ChatMessage{}, fmt.Errorf("prompt or messages not found in request body")
}

func GetPromptString(chatMessage common.ChatMessage) string {
	// If Text field is present, return text directly (for prompt format)
	if chatMessage.Text != "" {
		return chatMessage.Text
	}

	// For chat messages, convert to ChatML format
	result := ""
	for _, msg := range chatMessage.Messages {
		result += fmt.Sprintf("<|im_start|>%s\n%s<|im_end|>\n", msg.Role, msg.Content)
	}
	return result
}

func LoadEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		klog.Warningf("environment variable %s is not set, using default value: %s", key, defaultValue)
		return defaultValue
	}
	return value
}
