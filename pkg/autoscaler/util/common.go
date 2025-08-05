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

package util

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

func GetCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}

func SecondToTimestamp(sec int64) int64 {
	return sec * 1000
}

func IsRequestSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

func IsPodFailed(pod *corev1.Pod) bool {
	status := pod.Status
	metaData := pod.ObjectMeta
	return status.Phase == corev1.PodFailed || metaData.DeletionTimestamp != nil
}

func ExtractKeysToSet[K comparable, V any](m map[K]V) map[K]struct{} {
	set := make(map[K]struct{})
	for key := range m {
		set[key] = struct{}{}
	}
	return set
}
