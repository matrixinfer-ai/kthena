// Model server scheduling
package scheduler

import (
	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

type Scheduler interface {
	Schedule(req map[string]interface{}, pods []*datastore.PodInfo, pdGroup *aiv1alpha1.PDGroup) (*framework.Context, error)
	RunPostHooks(ctx *framework.Context, index int)
}
