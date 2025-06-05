// Model server scheduling
package scheduler

import (
	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

type Scheduler interface {
	Schedule(req map[string]interface{}, pods []*datastore.PodInfo, pdGroup *aiv1alpha1.PDGroup) (*TargetPods, error)
}

type TargetPods struct {
	// Decode pod in case of PD disaggregation
	PrimaryPod *datastore.PodInfo

	// Prefill pod in case of PD disaggregation.
	// Otherwise, it's nil
	PrefillPod *datastore.PodInfo
}
