package deploy

import (
	"context"
	"fmt"
	"io"
	"os"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
)

func access(manifest model.Deployment) error {
	for _, node := range manifest.Nodes {
		for _, path := range []string{node.SSH.IdentityFile, node.SSH.KnownHostsFile} {
			if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("prepare verified SSH access under ~/.ssh-custom")
			}
		}
	}
	return nil
}

func Run(ctx context.Context, directory string, runner process.Runner, progress io.Writer) error {
	return RunConcurrent(ctx, directory, runner, progress, 1)
}

// RunConcurrent applies up to jobs independent nodes. A node becomes eligible
// only after all of its downstream nodes, including their cleanup, succeed.
// The runner must support concurrent calls; progress is written by one coordinator.
func RunConcurrent(ctx context.Context, directory string, runner process.Runner, progress io.Writer, jobs int) error {
	if jobs < 1 || jobs > 64 {
		return fmt.Errorf("deployment jobs must be between 1 and 64")
	}
	lock, err := bundle.Lock(directory)
	if err != nil {
		return err
	}
	defer lock.Close()
	manifest, err := bundle.Load(directory)
	if err != nil {
		return err
	}
	if err := access(manifest); err != nil {
		return err
	}
	dependencies, err := bundle.Dependencies(directory, manifest)
	if err != nil {
		return err
	}
	return runFleet(ctx, directory, manifest, dependencies, runner, progress, jobs)
}
