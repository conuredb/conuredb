package config

import (
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config defines runtime configuration loaded from YAML and/or flags.
type Config struct {
	NodeID         string        `yaml:"node_id"`
	DataDir        string        `yaml:"data_dir"`
	RaftAddr       string        `yaml:"raft_addr"`
	HTTPAddr       string        `yaml:"http_addr"`
	Bootstrap      bool          `yaml:"bootstrap"`
	BarrierTimeout time.Duration `yaml:"barrier_timeout"`
}

// Load reads a YAML config file from path. If path is empty or the file
// does not exist, returns an empty Config and nil error.
func Load(path string) (Config, error) {
	var cfg Config
	if path == "" {
		return cfg, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close config file %q: %v\n", path, closeErr)
		}
	}()
	data, err := io.ReadAll(f)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
