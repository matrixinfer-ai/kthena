package framework

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

// Context stores information which maybe useful in Filter or Score plugins.
type Context struct {
	Model  string
	Prompt string

	Hashes []uint64

	DecodePod  *datastore.PodInfo
	PrefillPod *datastore.PodInfo
}

// FilterPlugin is an interface that is used to filter valid pods that can be sent request to.
type FilterPlugin interface {
	Name() string
	Filter(ctx *Context, pods []*datastore.PodInfo) []*datastore.PodInfo
}

// ScorePlugin is an interface that is used to rank pods that have passed the filter plugins.
// Note each plugin should generate score for a pod within [0, 100]
type ScorePlugin interface {
	Name() string
	Score(ctx *Context, pods []*datastore.PodInfo) map[*datastore.PodInfo]int
}

// PostHook is an interface that is executed after the scheduling is complete.
type PostHook interface {
	Name() string
	PostSchedule(ctx *Context)
}
