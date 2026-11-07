package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func runFleet(ctx context.Context, directory string, manifest model.Deployment, dependencies map[string][]string, runner process.Runner, progress io.Writer, jobs int) error {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	fleet := fleet{ctx: child, cancel: cancel, directory: directory, manifest: manifest,
		runner: runner, progress: progress, jobs: jobs, schedule: newSchedule(manifest.Order, dependencies),
		results: map[string]string{}, finished: make(chan nodeResult, jobs)}
	for _, id := range manifest.Order {
		fleet.results[id] = "not started"
	}
	if err := fleet.save(); err != nil {
		return err
	}
	return fleet.execute(ctx)
}

func (fleet *fleet) execute(parent context.Context) error {
	canceled := parent.Done()
	for fleet.completed < len(fleet.manifest.Order) {
		if parent.Err() != nil && fleet.failure == nil {
			fleet.stop(parent.Err())
			canceled = nil
		}
		select {
		case result := <-fleet.finished:
			fleet.finish(result)
			continue
		default:
		}
		if fleet.failure == nil {
			fleet.admit()
		}
		if fleet.active == 0 {
			if fleet.failure == nil {
				fleet.stop(fmt.Errorf("deployment has unfinished nodes without eligible work"))
			}
			break
		}
		select {
		case result := <-fleet.finished:
			fleet.finish(result)
		case <-canceled:
			fleet.stop(parent.Err())
			canceled = nil
		}
	}
	return errors.Join(fleet.failure, parent.Err())
}

func (fleet *fleet) admit() {
	nodes := fleet.schedule.take(fleet.jobs - fleet.active)
	if len(nodes) == 0 {
		return
	}
	for _, id := range nodes {
		fleet.results[id] = "applying; outcome not yet known"
	}
	if err := fleet.save(); err != nil {
		for _, id := range nodes {
			fleet.results[id] = "not started"
		}
		fleet.stop(err)
		return
	}
	for _, id := range nodes {
		fleet.active++
		go func() {
			err := applyNode(fleet.ctx, fleet.runner, fleet.directory, id, fleet.manifest)
			fleet.finished <- nodeResult{id: id, err: err}
		}()
	}
}

func (fleet *fleet) finish(result nodeResult) {
	results := []nodeResult{result}
	for len(results) < fleet.active {
		select {
		case next := <-fleet.finished:
			results = append(results, next)
		default:
			fleet.publish(results)
			return
		}
	}
	fleet.publish(results)
}

func (fleet *fleet) publish(results []nodeResult) {
	for _, result := range results {
		fleet.record(result)
	}
	if err := fleet.save(); err != nil {
		fleet.stop(err)
	}
	for _, result := range results {
		if result.err == nil {
			if _, err := fmt.Fprintf(fleet.progress, "%s: applied\n", result.id); err != nil {
				fleet.stop(fmt.Errorf("deployment progress: %w", err))
				return
			}
		}
	}
}

func (fleet *fleet) record(result nodeResult) {
	fleet.active--
	fleet.completed++
	if result.err == nil {
		fleet.results[result.id] = "applied; end-to-end verification pending"
		fleet.schedule.complete(result.id)
	} else {
		fleet.results[result.id] = "failed or cleanup failed; node may already be applied"
		fleet.stop(fmt.Errorf("deployment stopped at %s: %w; see deploy-result.json", result.id, result.err))
	}
}

func (fleet *fleet) save() error {
	if err := safeio.JSON(filepath.Join(fleet.directory, "deploy-result.json"), fleet.results); err != nil {
		return fmt.Errorf("deployment result could not be saved: %w", err)
	}
	return nil
}

func (fleet *fleet) stop(err error) {
	fleet.failure = errors.Join(fleet.failure, err)
	fleet.cancel()
}
