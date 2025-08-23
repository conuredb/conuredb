package raftnode

import (
	"fmt"
	"io"
	"os"

	"github.com/conure-db/conure-db/db"
	"github.com/hashicorp/raft"
)

type FSM struct {
	DB *db.DB
}

func (f *FSM) Apply(l *raft.Log) interface{} {
	cmd, err := DecodeCommand(l.Data)
	if err != nil {
		return err
	}
	switch cmd.Type {
	case CmdPut:
		return f.DB.Put(cmd.Key, cmd.Value)
	case CmdDelete:
		return f.DB.Delete(cmd.Key)
	default:
		return nil
	}
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &dbSnapshot{db: f.DB}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close ReadCloser during restore: %v\n", closeErr)
		}
	}()
	return f.DB.RestoreFrom(rc)
}

type dbSnapshot struct {
	db *db.DB
}

func (s *dbSnapshot) Persist(sink raft.SnapshotSink) error {
	defer func() {
		// Ensure sink is closed on any path
		_ = sink.Close()
	}()
	if err := s.db.SnapshotTo(sink); err != nil {
		_ = sink.Cancel()
		return err
	}
	return nil
}

func (s *dbSnapshot) Release() {}
