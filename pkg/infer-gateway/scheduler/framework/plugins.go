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
	"sync"
)

var (
	pluginMutex sync.RWMutex

	scorePluginBuilders  = map[string]ScorePluginFactory{}
	filterPluginBuilders = map[string]FilterPluginFactory{}
)

type ScorePluginFactory = func(arg map[string]interface{}) ScorePlugin
type FilterPluginFactory = func(arg map[string]interface{}) FilterPlugin

func RegisterScorePluginBuilder(name string, sp ScorePluginFactory) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	scorePluginBuilders[name] = sp
}

func GetScorePluginBuilder(name string) (ScorePluginFactory, bool) {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	sp, exist := scorePluginBuilders[name]
	return sp, exist
}

func RegisterFilterPluginBuilder(name string, fp FilterPluginFactory) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	filterPluginBuilders[name] = fp
}

func GetFilterPluginBuilder(name string) (FilterPluginFactory, bool) {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	fp, exist := filterPluginBuilders[name]
	return fp, exist
}
