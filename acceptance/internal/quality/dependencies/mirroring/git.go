package mirroring

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"vpn-script/internal/quality/dependencies/commands"
)

func repositoryPath(source Source) string {
	return filepath.Join("build/dependencies/mirrors", source.Name, "repository.git")
}

func sourceRef(source Source) string {
	return "refs/heads/nhop-source-" + source.Commit
}

func git(ctx context.Context, run commands.Runner, directory string, args ...string) (string, error) {
	base := []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_COUNT=0", "GIT_SSL_NO_VERIFY=false",
		"git", "-c", "protocol.file.allow=never", "-c", "protocol.ext.allow=never", "-c", "core.hooksPath=/dev/null", "-C", directory}
	raw, err := run(ctx, "env", append(base, args...)...)
	return strings.TrimSpace(string(raw)), err
}

func inspect(ctx context.Context, run commands.Runner, source Source) (Snapshot, error) {
	result := Snapshot{Source: source}
	directory := repositoryPath(source)
	checks := []struct {
		args     []string
		expected string
	}{
		{[]string{"rev-parse", "--is-bare-repository"}, "true"},
		{[]string{"rev-parse", "--is-shallow-repository"}, "false"},
		{[]string{"remote", "get-url", "upstream"}, source.URL},
		{[]string{"rev-parse", sourceRef(source) + "^{commit}"}, source.Commit},
	}
	for _, check := range checks {
		got, err := git(ctx, run, directory, check.args...)
		if err != nil || got != check.expected {
			return result, fmt.Errorf("source snapshot differs at %s: %v", check.args[0], err)
		}
	}
	tree, err := git(ctx, run, directory, "rev-parse", source.Commit+"^{tree}")
	if err != nil || !commitPattern.MatchString(tree) {
		return result, fmt.Errorf("invalid source tree identity: %v", err)
	}
	count, err := git(ctx, run, directory, "rev-list", "--count", source.Commit)
	if err != nil {
		return result, err
	}
	result.Tree, result.Ancestors = tree, 0
	result.Ancestors, err = strconv.Atoi(count)
	if err != nil || result.Ancestors < 1 {
		return result, fmt.Errorf("invalid source ancestry count: %v", err)
	}
	_, err = git(ctx, run, directory, "fsck", "--full")
	return result, err
}
