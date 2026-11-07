package deploy

import (
	"context"
	"io"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
)

type nodeResult struct {
	id  string
	err error
}

type nodeQueue []int

type deploymentSchedule struct {
	order      []string
	ready      nodeQueue
	waiting    map[string]int
	dependents map[string][]int
}

// The coordinator owns all mutable state. Workers share only the validated
// manifest, their context and the Runner, which must support concurrent calls.
type fleet struct {
	ctx       context.Context
	cancel    context.CancelFunc
	directory string
	manifest  model.Deployment
	runner    process.Runner
	progress  io.Writer
	schedule  deploymentSchedule
	jobs      int
	active    int
	completed int
	results   map[string]string
	finished  chan nodeResult
	failure   error
}
