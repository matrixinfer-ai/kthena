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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	GPUCacheUsage     = "gpu_usage"
	RequestWaitingNum = "request_waiting_num"
	TPOT              = "TPOT"
	TTFT              = "TTFT"
)

func GetNamespaceName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func GetPrompt(body map[string]interface{}) (string, error) {
	if prompt, ok := body["prompt"]; ok {
		res, ok := prompt.(string)
		if !ok {
			return "", fmt.Errorf("prompt is not a string")
		}
		return res, nil
	}

	if messages, ok := body["messages"]; ok {
		messageList, ok := messages.([]interface{})
		if !ok {
			return "", fmt.Errorf("messages is not a list")
		}

		res := ""
		for _, message := range messageList {
			msgMap, ok := message.(map[string]interface{})
			if !ok {
				continue
			}

			content, ok := msgMap["content"]
			if !ok {
				continue
			}

			contentStr, ok := content.(string)
			if !ok {
				continue
			}

			role, ok := msgMap["role"]
			if !ok {
				continue
			}

			roleStr, ok := role.(string)
			if !ok {
				continue
			}

			res += fmt.Sprintf("<|im_start|>%s\n%s<|im_end|>\n", roleStr, contentStr)
		}

		return res, nil
	}

	return "", fmt.Errorf("prompt or messages not found in request body")
}
