package checkout

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"vpn-script/internal/quality/dependencies/commands"
)

func Verify(ctx context.Context, run commands.Runner, root, revision string) error {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(revision) {
		return fmt.Errorf("source inventory requires a full reviewed Git commit")
	}
	head, err := run(ctx, "git", "-C", root, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(head)) != revision {
		return fmt.Errorf("source checkout revision mismatch: %v", err)
	}
	status, err := run(ctx, "git", "-C", root, "status", "--porcelain", "--untracked-files=all")
	if err != nil || len(status) != 0 {
		return fmt.Errorf("source inventory requires a clean source snapshot: %v", err)
	}
	return nil
}

func ReadIdentity(ctx context.Context, run commands.Runner, root, revision string) (Identity, error) {
	report := Identity{Revision: revision}
	remote, err := run(ctx, "git", "-C", root, "remote", "get-url", "origin")
	if err != nil {
		return report, err
	}
	report.Repository = strings.TrimSpace(string(remote))
	if !regexp.MustCompile(`^https://github\.com/Deliner/vpn-dep-[a-z0-9._-]+\.git$`).MatchString(report.Repository) {
		return report, fmt.Errorf("source must use the verified Deliner fork")
	}
	tree, err := run(ctx, "git", "-C", root, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return report, err
	}
	report.SourceTree = strings.TrimSpace(string(tree))
	return report, nil
}
