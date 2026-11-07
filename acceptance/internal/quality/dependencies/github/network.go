package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
)

func requestFork(ctx context.Context, run commands.Runner, upstream Repository, target string) (Repository, error) {
	name := strings.Split(target, "/")[1]
	raw, err := github(ctx, run, "--method", "POST", "repos/"+upstream.FullName+"/forks",
		"-f", "name="+name, "-F", "default_branch_only=false")
	if err != nil {
		return Repository{}, err
	}
	var result Repository
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	parts := strings.Split(result.FullName, "/")
	if result.ID <= 0 || len(parts) != 2 || !ownerPattern.MatchString(parts[0]) || !repoPattern.MatchString(parts[1]) {
		return result, fmt.Errorf("fork request returned an invalid repository identity")
	}
	return result, nil
}

func sharedFork(ctx context.Context, run commands.Runner, upstream, requested Repository, target string) (ForkRecord, error) {
	record := ForkRecord{Upstream: upstream, SharedNetwork: true}
	prefix := strings.Split(target, "/")[0] + "/vpn-dep-"
	if !strings.HasPrefix(strings.ToLower(requested.FullName), strings.ToLower(prefix)) {
		return record, fmt.Errorf("fork request selected unmanaged destination %s", requested.FullName)
	}
	repo, found, err := repository(ctx, run, requested.FullName)
	if err != nil || !found {
		return record, fmt.Errorf("shared fork unavailable: %v", err)
	}
	if err := verifySharedNetwork(upstream, repo, requested); err != nil {
		return record, err
	}
	record.Fork = repo
	return record, nil
}

func verifySharedNetwork(upstream, fork, requested Repository) error {
	rootID := upstream.ID
	if upstream.Source != nil {
		rootID = upstream.Source.ID
	}
	if !fork.Fork || fork.ID != requested.ID || !strings.EqualFold(fork.FullName, requested.FullName) || fork.Source == nil || rootID <= 0 || fork.Source.ID != rootID {
		return fmt.Errorf("GitHub selected %s outside the expected fork network", requested.FullName)
	}
	return nil
}

func reuseShared(ctx context.Context, run commands.Runner, upstream Repository, name, target string) (ForkRecord, bool, error) {
	var saved ForkRecord
	_, err := records.Read(name, &saved)
	if os.IsNotExist(err) || err == nil && !saved.SharedNetwork {
		return saved, false, nil
	}
	if err != nil {
		return saved, false, err
	}
	if saved.Upstream.ID != upstream.ID {
		return saved, false, fmt.Errorf("shared-network record belongs to another upstream ID")
	}
	record, err := sharedFork(ctx, run, upstream, saved.Fork, target)
	return record, true, err
}
