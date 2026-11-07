package mirroring

import (
	"context"
	"fmt"

	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/github"
	"vpn-script/internal/quality/dependencies/records"
)

func Publish(ctx context.Context, run commands.Runner, name string) (Snapshot, error) {
	prepared, err := preparedSource(ctx, run, name)
	if err != nil {
		return prepared, err
	}
	repo, err := github.EnsureMirror(ctx, run, name, prepared.Source.URL)
	if err != nil {
		return prepared, err
	}
	if prepared.Repository.ID != 0 && prepared.Repository.ID != repo.ID {
		return prepared, fmt.Errorf("GitHub mirror repository ID changed")
	}
	prepared.Repository = repo
	if err := records.Save("mirror-"+name+".json", prepared); err != nil {
		return prepared, err
	}
	if err := github.DisableMirrorActions(ctx, run, repo); err != nil {
		return prepared, err
	}
	if err := pushSource(ctx, run, prepared); err != nil {
		return prepared, err
	}
	prepared.Published = true
	return prepared, records.Save("mirror-"+name+".json", prepared)
}

func preparedSource(ctx context.Context, run commands.Runner, name string) (Snapshot, error) {
	source, err := readSource(name)
	if err != nil {
		return Snapshot{}, err
	}
	var prepared Snapshot
	if _, err := records.Read("mirror-"+name+".json", &prepared); err != nil {
		return prepared, err
	}
	if prepared.Source != source {
		return prepared, fmt.Errorf("reviewed source changed after preparation")
	}
	current, err := inspect(ctx, run, source)
	if err != nil {
		return prepared, err
	}
	if current.Tree != prepared.Tree || current.Ancestors != prepared.Ancestors {
		return prepared, fmt.Errorf("source snapshot changed after preparation")
	}
	return prepared, nil
}
