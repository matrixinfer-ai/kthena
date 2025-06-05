package plugins

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

var _ framework.FilterPlugin = &PrefillFilter{}

type PrefillFilter struct {
	name string

	prefillLabels   map[string]string
	inferGroupKey   string
	inferGroupValue string
}

func NewPrefillFilter(prefillLabels map[string]string, inferGroupKey string, inferGroupValue string) *PrefillFilter {
	return &PrefillFilter{
		name:            "prefill-filter",
		prefillLabels:   prefillLabels,
		inferGroupKey:   inferGroupKey,
		inferGroupValue: inferGroupValue,
	}
}

func (p *PrefillFilter) Name() string {
	return p.name
}

func (p *PrefillFilter) Filter(pods []*datastore.PodInfo, ctx *framework.Context) []*datastore.PodInfo {
	if len(p.prefillLabels) == 0 {
		// NOTE: prefill labels could not be empty when PD disaggregation is enabled.
		return nil
	}

	filtered := make([]*datastore.PodInfo, 0, len(pods))
	for _, pod := range pods {
		if pod.Pod.Labels == nil {
			continue
		}

		// Make sure prefill pod is in the same infer group of decode pod
		if pod.Pod.Labels[p.inferGroupKey] != p.inferGroupValue {
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

var _ framework.FilterPlugin = &DecodeFilter{}

type DecodeFilter struct {
	name string

	decodeLabels map[string]string
}

func NewDecodeFilter(decodeLabels map[string]string) *DecodeFilter {
	return &DecodeFilter{
		name:         "decode-filter",
		decodeLabels: decodeLabels,
	}
}

func (d *DecodeFilter) Name() string {
	return d.name
}

func (d *DecodeFilter) Filter(pods []*datastore.PodInfo, ctx *framework.Context) []*datastore.PodInfo {
	if len(d.decodeLabels) == 0 {
		// NOTE: decode labels could be empty when PD disaggragation is enabled.
		// Then we should return all pods.
		return pods
	}

	filtered := make([]*datastore.PodInfo, 0, len(pods))
	for _, pod := range pods {
		if pod.Pod.Labels == nil {
			continue
		}

		match := true
		for k, v := range d.decodeLabels {
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
