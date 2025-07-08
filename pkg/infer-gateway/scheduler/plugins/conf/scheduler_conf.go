package conf

type KubeSchedulerConfiguration struct {
	Profiles []Profile `yaml:"profiles"`
}

type Profile struct {
	PluginConfig []PluginConfig `yaml:"pluginConfig"`
	Plugins      *Plugins       `yaml:"plugins"`
}

type Plugins struct {
	PreFilter *PreFilter `yaml:"preFilter"`
	PreScore  *PreScore  `yaml:"preScore"`
}

type PreFilter struct {
	Enabled []string `yaml:"enabled"`
}

type PreScore struct {
	Enabled []PluginWithWeight `yaml:"enabled"`
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
