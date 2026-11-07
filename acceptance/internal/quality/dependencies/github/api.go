package github

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"vpn-script/internal/quality/dependencies/commands"
)

var ownerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

func forkName(upstream, owner string) (string, error) {
	parts := strings.Split(upstream, "/")
	if len(parts) != 2 || !ownerPattern.MatchString(parts[0]) || !repoPattern.MatchString(parts[1]) || !ownerPattern.MatchString(owner) {
		return "", fmt.Errorf("expected GitHub OWNER/REPOSITORY and personal account name")
	}
	if strings.EqualFold(parts[0], owner) {
		return "", fmt.Errorf("dependency is already owned by %s", owner)
	}
	name := "vpn-dep-" + strings.ToLower(strings.Join(parts, "-"))
	if len(name) > 100 {
		return "", fmt.Errorf("generated fork name exceeds GitHub limit")
	}
	return name, nil
}

func github(ctx context.Context, run commands.Runner, args ...string) ([]byte, error) {
	return run(ctx, "gh", append([]string{"api", "--hostname", "github.com"}, args...)...)
}

func repository(ctx context.Context, run commands.Runner, name string) (Repository, bool, error) {
	raw, err := github(ctx, run, "repos/"+name)
	if err != nil {
		var response struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(raw, &response) == nil && response.Status == "404" {
			return Repository{}, false, nil
		}
		return Repository{}, false, err
	}
	var repo Repository
	if err := json.Unmarshal(raw, &repo); err != nil {
		return repo, false, err
	}
	parts := strings.Split(repo.FullName, "/")
	if repo.ID <= 0 || len(parts) != 2 || !ownerPattern.MatchString(parts[0]) || !repoPattern.MatchString(parts[1]) {
		return repo, false, fmt.Errorf("GitHub returned unexpected repository identity for %s", name)
	}
	// GitHub resolves transferred/renamed upstreams to their canonical identity.
	// Destination ownership and the exact parent ID are checked by verifyFork.
	return repo, true, nil
}

func verifyFork(upstream, fork Repository, target string) error {
	if !fork.Fork || fork.Parent == nil || !strings.EqualFold(fork.FullName, target) {
		return fmt.Errorf("existing %s is not the requested fork", target)
	}
	if fork.Parent.ID != upstream.ID {
		return fmt.Errorf("existing %s has a different parent; refusing to change it", target)
	}
	return nil
}
