package config

const (
	DefaultMinIOS    = "16.0"
	DefaultConfig    = "debug"
	DefaultSimulator = "iPhone 16 Pro"
	DefaultBuildType = "simple"
)

// applyDefaults fills in zero-value fields with sensible defaults.
func applyDefaults(cfg *ProjectConfig) {
	if cfg.Defaults.Config == "" {
		cfg.Defaults.Config = DefaultConfig
	}
	if cfg.Defaults.Simulator == "" {
		cfg.Defaults.Simulator = DefaultSimulator
	}

	for i := range cfg.Targets {
		t := &cfg.Targets[i]
		if t.MinIOS == "" {
			t.MinIOS = DefaultMinIOS
		}
		if t.ProductType == "" {
			t.ProductType = ProductApp
		}
		if len(t.Sources) == 0 {
			t.Sources = []string{"Sources/"}
		}
		if t.Version == "" {
			t.Version = "1.0.0"
		}
		if t.BuildNum == "" {
			t.BuildNum = "1"
		}
	}
}
