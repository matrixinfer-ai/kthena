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
	// In non PD disaggregation case, the real target pod
	DecodePod *datastore.PodInfo

	// Prefill pod in case of PD disaggregation.
	// Otherwise, it's nil
	PrefillPod *datastore.PodInfo
}
