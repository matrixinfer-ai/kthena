package conf

type SchedulerConfiguration struct {
	PluginConfig []PluginConfig `yaml:"pluginConfig"`
	Plugins      Plugins        `yaml:"plugins"`
}

type Plugins struct {
	Filter Filter `yaml:"Filter"`
	Score  Score  `yaml:"Score"`
}

type Filter struct {
	Enabled  []string `yaml:"enabled"`
	Disabled []string `yaml:"disabled"`
}

type Score struct {
	Enabled  []PluginWithWeight `yaml:"enabled"`
	Disabled []PluginWithWeight `yaml:"disabled"`
}

type PluginWithWeight struct {
	Name   string `yaml:"name"`
	Weight int    `yaml:"weight"`
}

type PluginConfig struct {
	Name string `yaml:"name"`
	Args any    `yaml:"args,omitempty"`
}

type LeastRequestArgs struct {
	MaxWaitingRequests int `yaml:"maxWaitingRequests,omitempty"`
}

type LeastLatencyArgs struct {
	TTFTTPOTWeightFactor float64 `yaml:"TTFTTPOTWeightFactor,omitempty"`
}

type PrefixCacheArgs struct {
	BlockSizeToHash  int `yaml:"blockSizeToHash,omitempty"`
	MaxBlocksToMatch int `yaml:"maxBlocksToMatch,omitempty"`
	MaxHashCacheSize int `yaml:"maxHashCacheSize,omitempty"`
}
