package cleanup

import (
	"fmt"
	"path/filepath"

	"vpn-script/internal/quality/dependencies/github"
	"vpn-script/internal/quality/dependencies/mirroring"
	"vpn-script/internal/quality/dependencies/records"
)

func recordedTargets() (map[int64]Target, error) {
	result := map[int64]Target{}
	for _, pattern := range []string{"vpn-dep-*.json", "mirror-vpn-dep-*.json"} {
		paths, err := filepath.Glob(filepath.Join("build/dependencies", pattern))
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			target, err := readTarget(filepath.Base(path))
			if err != nil {
				return nil, err
			}
			if previous, found := result[target.ID]; found && (previous.Name != target.Name || previous.ParentID != target.ParentID || previous.SourceID != target.SourceID) {
				return nil, fmt.Errorf("conflicting saved identity for %s", target.Name)
			}
			result[target.ID] = target
		}
	}
	return result, nil
}

func readTarget(name string) (Target, error) {
	target := Target{Record: name}
	var repo github.Repository
	var err error
	if len(name) >= 7 && name[:7] == "mirror-" {
		var saved mirroring.Snapshot
		target.RecordSHA, err = records.Read(name, &saved)
		if !saved.Published {
			return target, fmt.Errorf("unpublished mirror cannot be a deletion target")
		}
		repo = saved.Repository
	} else {
		var saved github.ForkRecord
		target.RecordSHA, err = records.Read(name, &saved)
		repo = saved.Fork
	}
	if err != nil {
		return target, err
	}
	target.ID, target.Name, target.Fork, target.Description = repo.ID, repo.FullName, repo.Fork, repo.Description
	if repo.Parent != nil {
		target.ParentID = repo.Parent.ID
	}
	if repo.Source != nil {
		target.SourceID = repo.Source.ID
	}
	if target.ID <= 0 || !managedName.MatchString(target.Name) || repo.Private {
		return target, fmt.Errorf("invalid session record identity: %s", name)
	}
	return target, nil
}
