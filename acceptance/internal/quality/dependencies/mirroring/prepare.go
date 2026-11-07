package mirroring

import (
	"context"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
)

func Prepare(ctx context.Context, run commands.Runner, name string) (Snapshot, error) {
	source, err := readSource(name)
	if err != nil {
		return Snapshot{}, err
	}
	parent := "build/dependencies/mirrors"
	for _, directory := range []string{"build", "build/dependencies", parent} {
		if err := safeio.PrivateDir(directory); err != nil {
			return Snapshot{}, err
		}
	}
	if err := os.Mkdir(filepath.Join(parent, source.Name), 0700); err != nil {
		return Snapshot{}, err
	}
	directory := repositoryPath(source)
	if _, err := git(ctx, run, filepath.Dir(directory), "init", "--bare", "repository.git"); err != nil {
		return Snapshot{}, err
	}
	for _, args := range [][]string{{"remote", "add", "upstream", source.URL}, {"fetch", "--no-tags", "upstream", source.Commit}, {"update-ref", sourceRef(source), source.Commit}} {
		if _, err := git(ctx, run, directory, args...); err != nil {
			return Snapshot{}, err
		}
	}
	result, err := inspect(ctx, run, source)
	if err != nil {
		return result, err
	}
	return result, records.Save("mirror-"+source.Name+".json", result)
}
