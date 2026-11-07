package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"vpn-script/internal/quality/dependencies/commands"
)

// EnsureMirror creates an empty public repository for a reviewed external Git
// source. Matching names alone never authorize reuse of an existing repository.
func EnsureMirror(ctx context.Context, run commands.Runner, name, source string) (Repository, error) {
	address, err := url.Parse(source)
	if !repoPattern.MatchString(name) || !strings.HasPrefix(name, "vpn-dep-") || err != nil || address.Scheme != "https" || address.Hostname() == "" || address.User != nil || address.RawQuery != "" || address.Fragment != "" {
		return Repository{}, fmt.Errorf("invalid managed source mirror identity")
	}
	login, err := github(ctx, run, "user", "--jq", ".login")
	if err != nil || strings.TrimSpace(string(login)) != "Deliner" {
		return Repository{}, fmt.Errorf("authenticated personal account must be Deliner: %v", err)
	}
	target, description := "Deliner/"+name, "Source mirror of "+source+"; selected N-hop revisions."
	repo, found, err := repository(ctx, run, target)
	if err != nil {
		return repo, err
	}
	if !found {
		raw, err := github(ctx, run, "--method", "POST", "user/repos", "-f", "name="+name, "-f", "description="+description,
			"-F", "private=false", "-F", "auto_init=false", "-F", "has_issues=false", "-F", "has_projects=false", "-F", "has_wiki=false")
		if err != nil {
			return repo, err
		}
		if err := json.Unmarshal(raw, &repo); err != nil {
			return repo, err
		}
	}
	if repo.ID <= 0 || repo.FullName != target || repo.Fork || repo.Private || repo.Description != description {
		return repo, fmt.Errorf("existing repository is not the reviewed public source mirror")
	}
	return repo, nil
}

// DisableMirrorActions precedes the first push, so preserved upstream workflows
// cannot start merely because this source mirror is a new GitHub repository.
func DisableMirrorActions(ctx context.Context, run commands.Runner, repo Repository) error {
	if repo.ID <= 0 || !strings.HasPrefix(repo.FullName, "Deliner/vpn-dep-") || repo.Fork || repo.Private {
		return fmt.Errorf("expected owned public source mirror")
	}
	_, err := github(ctx, run, "--method", "PUT", "repos/"+repo.FullName+"/actions/permissions", "-F", "enabled=false")
	return err
}
