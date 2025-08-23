package raftnode

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

type Config struct {
	NodeID    string
	RaftAddr  string
	DataDir   string
	Bootstrap bool
}

type Node struct {
	raft *raft.Raft
	fsm  *FSM
}

func (n *Node) Raft() *raft.Raft {
	return n.raft
}

func (n *Node) IsLeader() bool {
	return n.raft.State() == raft.Leader
}

func (n *Node) Leader() raft.ServerAddress {
	return n.raft.Leader()
}

func (n *Node) AddVoter(id, addr string) error {
	future := n.raft.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 0)
	return future.Error()
}

func (n *Node) Apply(cmd Command, timeout time.Duration) error {
	b, err := EncodeCommand(cmd)
	if err != nil {
		return err
	}
	f := n.raft.Apply(b, timeout)
	return f.Error()
}

func StartNode(cfg Config, fsm *FSM) (*Node, error) {
	raftDir := filepath.Join(cfg.DataDir, "raft")
	if err := os.MkdirAll(raftDir, 0o755); err != nil {
		return nil, err
	}

	rcfg := raft.DefaultConfig()
	rcfg.LocalID = raft.ServerID(cfg.NodeID)
	rcfg.SnapshotInterval = 30 * time.Second
	rcfg.SnapshotThreshold = 8192

	// Stores
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "stable.bolt"))
	if err != nil {
		return nil, err
	}
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "log.bolt"))
	if err != nil {
		return nil, err
	}
	snaps, err := raft.NewFileSnapshotStore(raftDir, 3, os.Stderr)
	if err != nil {
		return nil, err
	}

	// Transport
	transport, err := raft.NewTCPTransport(cfg.RaftAddr, nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, err
	}

	r, err := raft.NewRaft(rcfg, fsm, logStore, stableStore, snaps, transport)
	if err != nil {
		return nil, err
	}

	n := &Node{raft: r, fsm: fsm}

	// Bootstrap if requested and no existing state
	if cfg.Bootstrap {
		hasState, err := raft.HasExistingState(logStore, stableStore, snaps)
		if err != nil {
			return nil, err
		}
		if !hasState {
			configuration := raft.Configuration{
				Servers: []raft.Server{{
					ID:      raft.ServerID(cfg.NodeID),
					Address: raft.ServerAddress(cfg.RaftAddr),
				}},
			}
			if err := r.BootstrapCluster(configuration).Error(); err != nil {
				return nil, err
			}
			log.Printf("bootstrapped single-node cluster: %s", cfg.NodeID)
		}
	}

	return n, nil
}
