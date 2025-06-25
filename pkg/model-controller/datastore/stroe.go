package datastore

import (
	"k8s.io/apimachinery/pkg/types"
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"sync"
)

// Store is an interface for storing and retrieving data
type Store interface {
}

type store struct {
	mutex sync.RWMutex

	model map[types.NamespacedName]*v1alpha1.Model
}

func New() (Store, error) {
	return &store{
		model: make(map[types.NamespacedName]*v1alpha1.Model),
	}, nil
}
