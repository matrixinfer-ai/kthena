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
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

const pdFilterPluginName = "pd-filter"

var _ framework.Plugin = &PDFilter{}

type PDFilter struct {
	name string

	decodeLabels  map[string]string
	prefillLabels map[string]string
	pdGroupKey    string
}

func NewPDFilter(decodeLabels map[string]string, prefillLabels map[string]string, pdGroupKey string) *PDFilter {
	return &PDFilter{
		name:          pdFilterPluginName,
		decodeLabels:  decodeLabels,
		prefillLabels: prefillLabels,
		pdGroupKey:    pdGroupKey,
	}
}

func (p *PDFilter) Name() string {
	return p.name
}

func (p *PDFilter) Filter(ctx *framework.Context, pods []*datastore.PodInfo) []*datastore.PodInfo {
	if ctx.DecodePod != nil {
		// Filter out prefill pods if decode pod is not nil.

		pdGroupValue := ctx.DecodePod.Pod.Labels[p.pdGroupKey]

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

	filtered := make([]*datastore.PodInfo, 0, len(pods))
	for _, pod := range pods {
		if pod.Pod.Labels == nil {
			continue
		}

		if _, ok := pod.Pod.Labels[p.pdGroupKey]; !ok {
			// Decode pod should have pd group key.
			continue
		}

		match := true
		for k, v := range p.decodeLabels {
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

func (p *PDFilter) Score(ctx *framework.Context, pods []*datastore.PodInfo) map[*datastore.PodInfo]int {
	scoreResults := make(map[*datastore.PodInfo]int)

	// Initialize all pods with score 0
	for _, pod := range pods {
		scoreResults[pod] = 0
	}
	return scoreResults
}
