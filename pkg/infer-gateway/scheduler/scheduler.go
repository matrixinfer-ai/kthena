// Model server scheduling
package scheduler

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

type Scheduler interface {
	Schedule(req map[string]interface{}, pods []datastore.PodInfo) (datastore.PodInfo, error)
}
