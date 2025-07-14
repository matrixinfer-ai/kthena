package framework

import (
	"sync"
)

var (
	pluginMutex sync.RWMutex

	scorePluginBuilders  = map[string]ScorePlugin{}
	filterPluginBuilders = map[string]FilterPlugin{}
)

func RegisterScorePluginBuilder(name string, sp ScorePlugin) {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	scorePluginBuilders[name] = sp
}

func GetScorePluginBuilder(name string) (ScorePlugin, bool) {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	sp, exist := scorePluginBuilders[name]
	return sp, exist
}

func RegisterFilterPluginBuilder(name string, fp FilterPlugin) {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	filterPluginBuilders[name] = fp
}

func GetFilterPluginBuilder(name string) (FilterPlugin, bool) {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()

	fp, exist := filterPluginBuilders[name]
	return fp, exist
}
