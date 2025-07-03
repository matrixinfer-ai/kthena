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

func GetPluginBuilder(name string) Plugin {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	p := pluginBuilders[name]
	return p
}
