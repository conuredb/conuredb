package raftnode

import "encoding/json"

type CommandType uint8

const (
	CmdPut CommandType = iota
	CmdDelete
)

type Command struct {
	Type  CommandType `json:"type"`
	Key   []byte      `json:"key"`
	Value []byte      `json:"value,omitempty"`
}

func EncodeCommand(cmd Command) ([]byte, error) {
	return json.Marshal(cmd)
}

func DecodeCommand(b []byte) (Command, error) {
	var c Command
	err := json.Unmarshal(b, &c)
	return c, err
}
