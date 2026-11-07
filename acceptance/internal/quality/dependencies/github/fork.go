package github

import (
	"context"
	"fmt"
	"strings"

	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
)

func Fork(ctx context.Context, run commands.Runner, upstreamName, owner string) (ForkRecord, error) {
	name, err := forkName(upstreamName, owner)
	if err != nil {
		return ForkRecord{}, err
	}
	upstream, err := authenticatedUpstream(ctx, run, upstreamName, owner)
	if err != nil {
		return ForkRecord{}, err
	}
	target := owner + "/" + name
	record, reused, err := reuseShared(ctx, run, upstream, name+".json", target)
	if err != nil {
		return record, err
	}
	if !reused {
		record, err = ensureFork(ctx, run, upstream, target)
	}
	if err != nil {
		return record, err
	}
	if err := records.Save(name+".json", record); err != nil {
		return record, err
	}
	return record, nil
}

func authenticatedUpstream(ctx context.Context, run commands.Runner, upstreamName, owner string) (Repository, error) {
	login, err := github(ctx, run, "user", "--jq", ".login")
	if err != nil {
		return Repository{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(string(login)), owner) {
		return Repository{}, fmt.Errorf("authenticated personal account must be %s", owner)
	}
	upstream, found, err := repository(ctx, run, upstreamName)
	if err != nil || !found {
		return Repository{}, fmt.Errorf("upstream %s unavailable: %v", upstreamName, err)
	}
	return upstream, nil
}

func ensureFork(ctx context.Context, run commands.Runner, upstream Repository, target string) (ForkRecord, error) {
	record := ForkRecord{Upstream: upstream}
	repo, found, err := repository(ctx, run, target)
	if err != nil {
		return record, err
	}
	if !found {
		requested, err := requestFork(ctx, run, upstream, target)
		if err != nil {
			return record, err
		}
		if !strings.EqualFold(requested.FullName, target) {
			return sharedFork(ctx, run, upstream, requested, target)
		}
		record.Created = true
		repo, found, err = repository(ctx, run, target)
		if err != nil || !found {
			return record, fmt.Errorf("fork requested; metadata not ready: retry verification: %v", err)
		}
	}
	if err := verifyFork(upstream, repo, target); err != nil {
		return record, err
	}
	record.Fork = repo
	return record, nil
}
