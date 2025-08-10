package main

import (
	"time"

	"github.com/conure-db/conure-db/pkg/config"
)

// CLIOverrides carries CLI-provided values. Empty strings mean "not set".
// For booleans, a pointer is used to detect if the flag was explicitly set.
type CLIOverrides struct {
	NodeID         string
	DataDir        string
	RaftAddr       string
	HTTPAddr       string
	Bootstrap      *bool
	BarrierTimeout *time.Duration
}

func mergeConfig(fileCfg config.Config, cli CLIOverrides) config.Config {
	cfg := fileCfg

	// Apply CLI overrides when provided
	if cli.NodeID != "" {
		cfg.NodeID = cli.NodeID
	}
	if cli.DataDir != "" {
		cfg.DataDir = cli.DataDir
	}
	if cli.RaftAddr != "" {
		cfg.RaftAddr = cli.RaftAddr
	}
	if cli.HTTPAddr != "" {
		cfg.HTTPAddr = cli.HTTPAddr
	}
	if cli.Bootstrap != nil {
		cfg.Bootstrap = *cli.Bootstrap
	}
	if cli.BarrierTimeout != nil {
		cfg.BarrierTimeout = *cli.BarrierTimeout
	}

	// Defaults for any still-empty values
	if cfg.NodeID == "" {
		cfg.NodeID = "node1"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	if cfg.RaftAddr == "" {
		cfg.RaftAddr = "127.0.0.1:7001"
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8081"
	}
	if cfg.BarrierTimeout == 0 {
		cfg.BarrierTimeout = 3 * time.Second
	}

	return cfg
}
