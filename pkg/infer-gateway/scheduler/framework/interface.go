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

package framework

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

// Context stores information which maybe useful in Filter or Score plugins.
type Context struct {
	Model  string
	UserIp string // Client IP address, useful for vtc plugins
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

// TokenCountablePlugin adds token count to the context.
type TokenCountablePlugin interface {
	ScorePlugin
	UpdateTokenCount(userIp string, inputTokens, outputTokens float64)
}

// PostHook is an interface that is executed after the scheduling is complete.
type PostHook interface {
	Name() string
	PostSchedule(ctx *Context)
}
