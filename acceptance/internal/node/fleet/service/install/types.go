// Package install publishes and controls the owned Fleet agent service. It
// retains credentials, replay journals and immutable versions on removal.
package install

import (
	"context"
	"os"
	"time"
)

type Options struct {
	State    string
	Interval time.Duration
}

type Receipt struct {
	Generation string `json:"generation"`
	Unit       string `json:"unit"`
	State      string `json:"state"`
}

type identity struct {
	Version         int    `json:"version"`
	State           string `json:"state"`
	IntervalSeconds int64  `json:"interval_seconds"`
	BinarySHA256    string `json:"binary_sha256"`
}

type revision struct {
	Identity   identity `json:"identity"`
	UnitSHA256 string   `json:"unit_sha256"`
}

type ownership struct {
	Version int    `json:"version"`
	Token   string `json:"token"`
}

type manager func(context.Context, ...string) ([]byte, error)

type store struct {
	base, unit string
	owner      int
	token      string
	lock       *os.File
}

type transaction struct {
	store   *store
	manager manager
}

const marker = "# nhop-fleet agent installation v1 "
