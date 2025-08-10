package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/conure-db/conure-db/db"
	"github.com/conure-db/conure-db/pkg/raftnode"
	"github.com/hashicorp/raft"
)

type Server struct {
	node           *raftnode.Node
	db             *db.DB
	barrierTimeout time.Duration
}

func New(node *raftnode.Node, db *db.DB) *Server {
	return &Server{node: node, db: db, barrierTimeout: 3 * time.Second}
}

func (s *Server) WithBarrierTimeout(d time.Duration) *Server {
	if d > 0 {
		s.barrierTimeout = d
	}
	return s
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/kv", s.handleKV)
	mux.HandleFunc("/join", s.handleJoin)
	mux.HandleFunc("/remove", s.handleRemove)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/raft/config", s.handleRaftConfig)
	mux.HandleFunc("/raft/stats", s.handleRaftStats)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"is_leader": s.node.IsLeader(),
		"leader":    string(s.node.Leader()),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleRaftConfig(w http.ResponseWriter, r *http.Request) {
	f := s.node.Raft().GetConfiguration()
	if err := f.Error(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	cfg := f.Configuration()
	type serverInfo struct {
		ID       string `json:"id"`
		Address  string `json:"address"`
		Suffrage string `json:"suffrage"`
	}
	resp := struct {
		Leader  string       `json:"leader"`
		Servers []serverInfo `json:"servers"`
	}{Leader: string(s.node.Leader())}
	for _, sv := range cfg.Servers {
		resp.Servers = append(resp.Servers, serverInfo{
			ID:       string(sv.ID),
			Address:  string(sv.Address),
			Suffrage: suffrageToString(sv.Suffrage),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func suffrageToString(s raft.ServerSuffrage) string {
	switch s {
	case raft.Voter:
		return "voter"
	case raft.Staging:
		return "staging"
	case raft.Nonvoter:
		return "nonvoter"
	default:
		return "unknown"
	}
}

func (s *Server) handleRaftStats(w http.ResponseWriter, r *http.Request) {
	stats := s.node.Raft().Stats()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	type req struct{ ID, RaftAddr string }
	var body req
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	if !s.node.IsLeader() {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"leader": string(s.node.Leader())})
		return
	}
	if err := s.node.AddVoter(body.ID, body.RaftAddr); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	type req struct{ ID string }
	var body req
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	if !s.node.IsLeader() {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"leader": string(s.node.Leader())})
		return
	}
	f := s.node.Raft().RemoveServer(raft.ServerID(body.ID), 0, 0)
	if err := f.Error(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *Server) handleKV(w http.ResponseWriter, r *http.Request) {
	key := []byte(r.URL.Query().Get("key"))
	if len(key) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing key"))
		return
	}

	// Refresh header to reflect external updates (e.g., local REPL)
	_ = s.db.Reload()

	switch r.Method {
	case http.MethodGet:
		stale := strings.EqualFold(r.URL.Query().Get("stale"), "true") || r.URL.Query().Get("stale") == "1"
		if s.node.IsLeader() {
			// linearizable read via barrier
			barrier := s.node.Raft().Barrier(s.barrierTimeout)
			if err := barrier.Error(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(err.Error()))
				return
			}
			val, err := s.db.Get(key)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(err.Error()))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(val)
			return
		}
		// follower: serve stale read if requested; else indicate leader
		if stale {
			val, err := s.db.Get(key)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(err.Error()))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(val)
			return
		}
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"leader": string(s.node.Leader())})

	case http.MethodPut:
		if !s.node.IsLeader() {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"leader": string(s.node.Leader())})
			return
		}
		value, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		cmd := raftnode.Command{Type: raftnode.CmdPut, Key: key, Value: value}
		if err := s.node.Apply(cmd, 5*time.Second); err != nil {
			log.Printf("apply error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))

	case http.MethodDelete:
		if !s.node.IsLeader() {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"leader": string(s.node.Leader())})
			return
		}
		cmd := raftnode.Command{Type: raftnode.CmdDelete, Key: key}
		if err := s.node.Apply(cmd, 5*time.Second); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
