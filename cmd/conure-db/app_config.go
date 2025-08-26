package main

import (
	"flag"

	"github.com/conuredb/conuredb/pkg/config"
)

// LoadEffectiveConfig defines CLI flags, parses the optional YAML config,
// applies CLI overrides, and returns the effective configuration.
func LoadEffectiveConfig() (config.Config, error) {
	var (
		configPath string
		nodeID     string
		dataDir    string
		raftAddr   string
		httpAddr   string
		bootstrap  settableBool
		barrier    settableDuration
	)

	flag.StringVar(&configPath, "config", "", "path to YAML config file")
	flag.StringVar(&nodeID, "node-id", "", "unique node ID")
	flag.StringVar(&dataDir, "data-dir", "", "data directory for node state")
	flag.StringVar(&raftAddr, "raft-addr", "", "raft bind/advertise address host:port")
	flag.StringVar(&httpAddr, "http-addr", "", "http bind address")
	flag.Var(&bootstrap, "bootstrap", "bootstrap single-node cluster if no existing state")
	flag.Var(&barrier, "barrier-timeout", "raft barrier timeout (e.g., 3s)")
	flag.Parse()

	cfgFile, err := config.Load(configPath)
	if err != nil {
		return config.Config{}, err
	}

	cli := CLIOverrides{
		NodeID:   nodeID,
		DataDir:  dataDir,
		RaftAddr: raftAddr,
		HTTPAddr: httpAddr,
	}
	if bootstrap.set {
		cli.Bootstrap = &bootstrap.val
	}
	if barrier.set {
		cli.BarrierTimeout = &barrier.val
	}

	cfg := mergeConfig(cfgFile, cli)
	return cfg, nil
}
