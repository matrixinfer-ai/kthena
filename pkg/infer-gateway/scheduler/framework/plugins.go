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
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	scorePluginBuilders[name] = sp
}

func GetScorePluginBuilder(name string) (ScorePlugin, bool) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	sp, exist := scorePluginBuilders[name]
	return sp, exist
}

func RegisterFilterPluginBuilder(name string, fp FilterPlugin) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	filterPluginBuilders[name] = fp
}

func GetFilterPluginBuilder(name string) (FilterPlugin, bool) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	fp, exist := filterPluginBuilders[name]
	return fp, exist
}
