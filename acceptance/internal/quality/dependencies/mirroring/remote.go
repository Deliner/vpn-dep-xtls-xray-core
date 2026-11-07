package mirroring

import (
	"context"
	"fmt"
	"strings"

	"vpn-script/internal/quality/dependencies/commands"
)

func pushSource(ctx context.Context, run commands.Runner, prepared Snapshot) error {
	source := prepared.Source
	address := "https://github.com/Deliner/" + source.Name + ".git"
	if err := checkRemoteRef(ctx, run, source, address, true); err != nil {
		return err
	}
	args := []string{"-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "--porcelain", address, source.Commit + ":" + sourceRef(source)}
	if _, err := git(ctx, run, repositoryPath(source), args...); err != nil {
		return err
	}
	return checkRemoteRef(ctx, run, source, address, false)
}

func checkRemoteRef(ctx context.Context, run commands.Runner, source Source, address string, allowMissing bool) error {
	value, err := git(ctx, run, repositoryPath(source), "ls-remote", address, sourceRef(source))
	if err != nil {
		return err
	}
	if value == "" && allowMissing {
		return nil
	}
	fields := strings.Fields(value)
	if len(fields) != 2 || fields[0] != source.Commit || fields[1] != sourceRef(source) {
		return fmt.Errorf("remote immutable source ref differs from the reviewed commit")
	}
	return nil
}
