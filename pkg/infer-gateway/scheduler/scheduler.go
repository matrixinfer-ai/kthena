// Model server scheduling
package scheduler

import (
	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

type Scheduler interface {
	Schedule(req map[string]interface{}, pods []*datastore.PodInfo, pdGroup *aiv1alpha1.PDGroup) ([]*framework.Context, error)
}

type TargetPods struct {
	// Decode pod in case of PD disaggregation
	// In non PD disaggregation case, the real target pod
	DecodePod *datastore.PodInfo

	// Prefill pod in case of PD disaggregation.
	// Otherwise, it's nil
	PrefillPod *datastore.PodInfo
}
