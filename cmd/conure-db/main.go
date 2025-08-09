package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/conure-db/conure-db/db"
	"github.com/conure-db/conure-db/pkg/api"
	"github.com/conure-db/conure-db/pkg/raftnode"
)

func main() {
	var (
		dataDir   = flag.String("data-dir", "./data", "data directory for node state")
		nodeID    = flag.String("node-id", "node1", "unique node ID")
		raftAddr  = flag.String("raft-addr", "127.0.0.1:7001", "raft bind/advertise address host:port")
		httpAddr  = flag.String("http-addr", ":8081", "http bind address")
		bootstrap = flag.Bool("bootstrap", true, "bootstrap single-node cluster if no existing state")
	)
	flag.Parse()

	// Suppress global logger output used by some dependencies; use our own logger instead
	log.SetOutput(io.Discard)
	appLog := log.New(os.Stdout, "", log.LstdFlags)

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		appLog.Fatalf("mkdir: %v", err)
	}

	dbPath := filepath.Join(*dataDir, "conure.db")
	store, err := db.Open(dbPath)
	if err != nil {
		appLog.Fatalf("open db: %v", err)
	}
	defer store.Close()

	fsm := &raftnode.FSM{DB: store}
	node, err := raftnode.StartNode(raftnode.Config{
		NodeID:    *nodeID,
		RaftAddr:  *raftAddr,
		DataDir:   *dataDir,
		Bootstrap: *bootstrap,
	}, fsm)
	if err != nil {
		appLog.Fatalf("start raft: %v", err)
	}

	mux := http.NewServeMux()
	api.New(node, store).Register(mux)
	appLog.Printf("conure-db running: http=%s raft=%s id=%s", *httpAddr, *raftAddr, *nodeID)
	fmt.Println("Endpoints: /kv (GET, PUT, DELETE), /join (POST), /status (GET)")
	if err := http.ListenAndServe(*httpAddr, mux); err != nil {
		appLog.Fatalf("http: %v", err)
	}
}
