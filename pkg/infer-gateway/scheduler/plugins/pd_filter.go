package plugins

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

var _ framework.FilterPlugin = &PDFilter{}

type PDFilter struct {
	name string

	decodeLabels  map[string]string
	prefillLabels map[string]string
	pdGroupKey    string
}

func NewPDFilter(decodeLabels map[string]string, prefillLabels map[string]string, pdGroupKey string) *PDFilter {
	return &PDFilter{
		name:          "pd-filter",
		decodeLabels:  decodeLabels,
		prefillLabels: prefillLabels,
		pdGroupKey:    pdGroupKey,
	}
}

func (p *PDFilter) Name() string {
	return p.name
}

func (p *PDFilter) Filter(pods []*datastore.PodInfo, ctx *framework.Context) []*datastore.PodInfo {
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
