package datastore

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

// Store is an interface for storing and retrieving data
type Store interface {
	GetInferGroupByModelInfer(name types.NamespacedName) (int, *[]string, error)
	GetRunningPodByInferGroup(name types.NamespacedName) (int, *[]string, error)
	GetInferGroupStatus(name types.NamespacedName) string
	DeleteModelInfer(name types.NamespacedName) error
	DeleteInferGroup(name types.NamespacedName) error
	SetInferGroupForModelInfer(name types.NamespacedName, inferGroupList *[]string) error
	SetRunningPodForInferGroup(name types.NamespacedName, runningPodList *[]string) error
	SetInferGroupStatus(name types.NamespacedName, inferGroupStatus string) error
}

type store struct {
	mutex sync.RWMutex

	inferGroup             map[types.NamespacedName]*[]string
	runningPodOfInferGroup map[types.NamespacedName]*[]string
	inferGroupStatus       map[types.NamespacedName]string
}

func New() (Store, error) {
	return &store{
		inferGroup:             make(map[types.NamespacedName]*[]string),
		runningPodOfInferGroup: make(map[types.NamespacedName]*[]string),
		inferGroupStatus:       make(map[types.NamespacedName]string),
	}, nil
}

func (s *store) GetInferGroupByModelInfer(name types.NamespacedName) (int, *[]string, error) {
	// todo Returns the number of infergroups, the list of infergroup names, and errors for a modelinfer
	return 0, nil, nil
}

func (s *store) GetRunningPodByInferGroup(name types.NamespacedName) (int, *[]string, error) {
	// todo Returns the number of running pods, the list of running pod names, and errors for an infergroup
	return 0, nil, nil
}

func (s *store) GetInferGroupStatus(name types.NamespacedName) string {
	return ""
}

func (s *store) DeleteModelInfer(name types.NamespacedName) error {
	// delete modelInfer in inferGroup map
	return nil
}

func (s *store) DeleteInferGroup(name types.NamespacedName) error {
	// delete inferGroup in runningPodOfInferGroup and inferGroupStatus map
	return nil
}

func (s *store) SetInferGroupForModelInfer(name types.NamespacedName, inferGroupList *[]string) error {
	return nil
}

func (s *store) SetRunningPodForInferGroup(name types.NamespacedName, runningPodList *[]string) error {
	return nil
}

func (s *store) SetInferGroupStatus(name types.NamespacedName, inferGroupStatus string) error {
	return nil
}
