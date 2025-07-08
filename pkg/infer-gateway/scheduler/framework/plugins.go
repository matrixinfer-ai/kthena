package framework

import (
	"sync"
)

var (
	pluginMutex sync.RWMutex

	pluginBuilders = map[string]Plugin{}
)

func RegisterPluginBuilder(name string, p Plugin) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	pluginBuilders[name] = p
}

func GetPluginBuilder(name string) (Plugin, bool) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	p, exist := pluginBuilders[name]
	return p, exist
}
