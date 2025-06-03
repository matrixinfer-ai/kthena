package framework

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

// Context stores information which maybe useful in Filter or Score plugins.
type Context struct {
	Model string
	User string
	Message string
}

// FilterPlugin is an interface that is used to filter valid pods that can be sent request to.
type FilterPlugin interface {
	Name() string
	Filter(pods []*datastore.PodInfo, ctx *Context) []*datastore.PodInfo
}

// ScorePlugin is an interface that is used to rank pods that have passed the filter plugins.
// Note each plugin should generate score for a pod within [0, 100]
type ScorePlugin interface {
	Name() string
	Score(pods []*datastore.PodInfo, ctx *Context) map[*datastore.PodInfo]int
}
