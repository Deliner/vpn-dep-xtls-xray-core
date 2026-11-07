package release

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

// Permissions validates a candidate or reused release before publication.
func Permissions(directory string, data model.NodeConfig, owner, group int) error {
	return permissions(directory, names(data), owner, group)
}

// Verify checks the previous committed contents even when the desired manifest
// changes. An unsafe historical release must not become a rollback target.
func Verify(directory string, files map[string]string, owner, group int) error {
	names := dag.Keys(files)
	if len(names) == 0 {
		return fmt.Errorf("committed release has no file inventory")
	}
	if err := permissions(directory, names, owner, group); err != nil {
		return err
	}
	hashes, err := inventory(directory, names)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(hashes, files) {
		return fmt.Errorf("committed release contents differ: %s; inspect before retrying", directory)
	}
	return nil
}

func permissions(directory string, names []string, owner, group int) error {
	if err := safeio.CheckAccess(directory, os.ModeDir|0750, owner, group); err != nil {
		return err
	}
	for _, name := range names {
		mode, err := fileMode(name)
		if err != nil {
			return err
		}
		fileGroup := -1
		if mode == 0640 {
			fileGroup = group
		}
		if err := safeio.CheckAccess(filepath.Join(directory, name), mode, owner, fileGroup); err != nil {
			return err
		}
	}
	return nil
}

func fileMode(name string) (os.FileMode, error) {
	switch name {
	case "xray", "sing-box", "libcronet.so", "nhop-node":
		return 0755, nil
	case "config.json", "xray.json", "sing-box.json", "processes.json":
		return 0640, nil
	default:
		return 0, fmt.Errorf("unknown managed release file: %q", name)
	}
}
