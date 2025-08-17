package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/conure-db/conure-db/db"
	"github.com/conure-db/conure-db/pkg/api"
	"github.com/conure-db/conure-db/pkg/raftnode"
)

func main() {
	// Suppress global logger output used by some dependencies; use our own logger instead
	log.SetOutput(io.Discard)
	appLog := log.New(os.Stdout, "", log.LstdFlags)

	cfg, err := LoadEffectiveConfig()
	if err != nil {
		appLog.Fatalf("load config: %v", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		appLog.Fatalf("mkdir: %v", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "conure.db")
	store, err := db.Open(dbPath)
	if err != nil {
		appLog.Fatalf("open db: %v", err)
	}
	defer store.Close()

	fsm := &raftnode.FSM{DB: store}
	node, err := raftnode.StartNode(raftnode.Config{
		NodeID:    cfg.NodeID,
		RaftAddr:  cfg.RaftAddr,
		DataDir:   cfg.DataDir,
		Bootstrap: cfg.Bootstrap,
	}, fsm)
	if err != nil {
		appLog.Fatalf("start raft: %v", err)
	}

	// Auto-join when not bootstrapping
	if !cfg.Bootstrap {
		appLog.Printf("Starting auto-join process for node %s", cfg.NodeID)
		go joinCluster(cfg.NodeID, cfg.RaftAddr, 2*time.Second, 0)
	} else {
		appLog.Printf("Node %s is configured as bootstrap node", cfg.NodeID)
	}

	mux := http.NewServeMux()
	api.New(node, store).WithBarrierTimeout(cfg.BarrierTimeout).Register(mux)
	appLog.Printf("conure-db running: http=%s raft=%s id=%s", cfg.HTTPAddr, cfg.RaftAddr, cfg.NodeID)
	fmt.Println("Endpoints: /kv (GET, PUT, DELETE), /join (POST), /remove (POST), /status (GET), /raft/config, /raft/stats")
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		appLog.Fatalf("http: %v", err)
	}
}
