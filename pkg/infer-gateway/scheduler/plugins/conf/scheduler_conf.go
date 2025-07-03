package conf

var (
	ScorePluginMap = make(map[string]int)
	FilterPlugins  []string
	PluginsArgs    = make(map[string]PluginArgs)
)

type KubeSchedulerConfiguration struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Profiles   []Profile `yaml:"profiles"`
}

type Profile struct {
	SchedulerName string         `yaml:"schedulerName"`
	PluginConfig  []PluginConfig `yaml:"pluginConfig"`
	Plugins       *Plugins       `yaml:"plugins"`
}

type Plugins struct {
	PreFilter *PreFilter `yaml:"preFilter"`
	PreScore  *PreScore  `yaml:"preScore"`
}

type PreFilter struct {
	Enabled []PluginName `yaml:"enabled"`
}

type PreScore struct {
	Enabled []PluginWithWeight `yaml:"enabled"`
}

type PluginName struct {
	Name string `yaml:"name"`
}

type PluginWithWeight struct {
	Name   string `yaml:"name"`
	Weight int    `yaml:"weight"`
}

type PluginConfig struct {
	Name string     `yaml:"name"`
	Args PluginArgs `yaml:"args,omitempty"`
}

type PluginArgs struct {
	MaxWaitingRequests   int     `yaml:"maxWaitingRequests,omitempty"`
	MaxScore             float64 `yaml:"maxScore,omitempty"`
	TTFTTPOTWeightFactor float64 `yaml:"TTFTTPOTWeightFactor,omitempty"`
	BlockSizeToHash      int     `yaml:"blockSizeToHash,omitempty"`
	MaxBlocksToMatch     int     `yaml:"maxBlocksToMatch,omitempty"`
	MaxHashCacheSize     int     `yaml:"maxHashCacheSize,omitempty"`
}
