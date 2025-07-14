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
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	scorePluginBuilders[name] = sp
}

func GetScorePluginBuilder(name string) (ScorePluginFactory, bool) {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	sp, exist := scorePluginBuilders[name]
	return sp, exist
}

func RegisterFilterPluginBuilder(name string, fp FilterPluginFactory) {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	filterPluginBuilders[name] = fp
}

func GetFilterPluginBuilder(name string) (FilterPluginFactory, bool) {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	fp, exist := filterPluginBuilders[name]
	return fp, exist
}
