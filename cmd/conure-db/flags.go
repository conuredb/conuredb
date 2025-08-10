package main

import (
	"flag"
	"time"
)

type settableBool struct {
	set bool
	val bool
}

func (b *settableBool) Set(s string) error {
	b.set = true
	if s == "" {
		b.val = true
		return nil
	}
	v, err := parseBool(s)
	if err != nil {
		return err
	}
	b.val = v
	return nil
}

func (b *settableBool) String() string {
	if b == nil {
		return "false"
	}
	if b.set {
		if b.val {
			return "true"
		}
		return "false"
	}
	return "false"
}

func (b *settableBool) IsBoolFlag() bool { return true }

func parseBool(s string) (bool, error) {
	// Simple parser
	switch s {
	case "1", "t", "T", "true", "TRUE", "True":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False":
		return false, nil
	default:
		return false, flag.ErrHelp
	}
}

type settableDuration struct {
	set bool
	val time.Duration
}

func (d *settableDuration) Set(s string) error {
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.set = true
	d.val = v
	return nil
}

func (d *settableDuration) String() string {
	if d == nil {
		return "0s"
	}
	if d.set {
		return d.val.String()
	}
	return "0s"
}
