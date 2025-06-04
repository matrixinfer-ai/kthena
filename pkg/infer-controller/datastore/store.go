package datastore

import (
	"k8s.io/apimachinery/pkg/types"
)

// Store is an interface for storing and retrieving data
type Store interface {
	GetInferGroupByModelInfer(modelInferName types.NamespacedName) (int, []InferGroup, error)
	GetRunningPodByInferGroup(inferGroupName types.NamespacedName) (int, []string, error)
	GetInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string) string
	DeleteModelInfer(modelInferName types.NamespacedName) error
	DeleteInferGroupOfRunningPodMap(inferGroupName types.NamespacedName) error
	UpdateInferGroupForModelInfer(modelInferName types.NamespacedName, inferGroupList []InferGroup) error
	UpdateRunningPodForInferGroup(inferGroupName types.NamespacedName, runningPodList []string) error
	UpdateInferGroupStatus(modelInferName types.NamespacedName, inferGroupName, Status string) error
}

type store struct {
	// mutex sync.RWMutex

	inferGroup             map[types.NamespacedName][]InferGroup
	runningPodOfInferGroup map[types.NamespacedName][]string
}

type InferGroup struct {
	Name      string
	Namespace string
	Status    string
}

func New() (Store, error) {
	return &store{
		inferGroup:             make(map[types.NamespacedName][]InferGroup),
		runningPodOfInferGroup: make(map[types.NamespacedName][]string),
	}, nil
}

func (s *store) GetInferGroupByModelInfer(modelInferName types.NamespacedName) (int, []InferGroup, error) {
	// todo Returns the number of infergroups, the list of infergroup, and errors for a modelinfer
	return 0, nil, nil
}

func (s *store) GetRunningPodByInferGroup(inferGroupName types.NamespacedName) (int, []string, error) {
	// todo Returns the number of running pods, the list of running pod names, and errors for an infergroup
	return 0, nil, nil
}

func (s *store) GetInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string) string {
	return ""
}

func (s *store) DeleteModelInfer(modelInferName types.NamespacedName) error {
	// delete modelInfer in inferGroup map
	return nil
}

func (s *store) DeleteInferGroupOfRunningPodMap(inferGroupName types.NamespacedName) error {
	// delete inferGroup in runningPodOfInferGroup
	return nil
}

func (s *store) UpdateInferGroupForModelInfer(modelInferName types.NamespacedName, inferGroupList []InferGroup) error {
	return nil
}

func (s *store) UpdateRunningPodForInferGroup(inferGroupName types.NamespacedName, runningPodList []string) error {
	return nil
}

func (s *store) UpdateInferGroupStatus(modelInferName types.NamespacedName, inferGroupName, Status string) error {
	return nil
}
