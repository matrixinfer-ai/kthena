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

package plugins

import (
	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

type PDFilter struct {
	name string

	decodeLabels  map[string]string
	prefillLabels map[string]string
	pdGroupKey    string
}

func NewPDFilter(pdGroup *aiv1alpha1.PDGroup) *PDFilter {
	return &PDFilter{
		name:          "pd-filter",
		decodeLabels:  pdGroup.DecodeLabels,
		prefillLabels: pdGroup.PrefillLabels,
		pdGroupKey:    pdGroup.GroupKey,
	}
}

func (p *PDFilter) Name() string {
	return p.name
}

func (p *PDFilter) FilterPrefillInstances(ctx *framework.Context, pods []*datastore.PodInfo) []*datastore.PodInfo {
	if ctx.DecodePods != nil {
		// Filter out prefill pods if decode pod is not nil.

		pdGroupValue := ctx.DecodePods[ctx.PDIndex].Pod.Labels[p.pdGroupKey]

		filtered := make([]*datastore.PodInfo, 0, len(pods))
		for _, pod := range pods {
			if pod.Pod.Labels == nil {
				continue
			}

			// Make sure prefill pod is in the same infer group of decode pod
			if pod.Pod.Labels[p.pdGroupKey] != pdGroupValue {
				continue
			}

			match := true
			for k, v := range p.prefillLabels {
				if pod.Pod.Labels[k] != v {
					match = false
					break
				}
			}

			if match {
				filtered = append(filtered, pod)
			}
		}
		return filtered
	}
	return nil
}

// FilterDecodeInstances filters decode pods based on the PD group key and decode labels.
func (p *PDFilter) FilterDecodeInstances(ctx *framework.Context, pods []*datastore.PodInfo) []*datastore.PodInfo {
	// Early return for empty input
	if len(pods) == 0 {
		return pods
	}

	out := make([]*datastore.PodInfo, 0, len(pods))

	for _, pod := range pods {
		labels := pod.Pod.Labels
		if labels == nil {
			continue
		}

		// Check if pd group key exists (required for decode pods)
		if _, ok := labels[p.pdGroupKey]; !ok {
			continue
		}

		// Check all decode labels match
		match := true
		for k, v := range p.decodeLabels {
			if labels[k] != v {
				match = false
				break
			}
		}

		if match {
			out = append(out, pod)
		}
	}

	return out
}
