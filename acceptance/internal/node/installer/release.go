package installer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/artifact"
	"vpn-script/internal/node/release"
	"vpn-script/internal/node/serviceuser"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

func (i *Installer) serviceGroup() (int, error) {
	if !i.probe("id", "-u", account) {
		_, err := i.command("useradd", "--system", "--user-group", "--home-dir", serviceuser.Home,
			"--no-create-home", "--shell", serviceuser.Shell, account)
		if err != nil {
			return 0, err
		}
	}
	return i.existingGroup()
}

func groupDir(path string, group int) error {
	if err := safeio.NoSymlink(path); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0750); err != nil {
		return err
	}
	if err := os.Chown(path, 0, group); err != nil {
		return err
	}
	return os.Chmod(path, 0750)
}

func (i *Installer) prepare(data model.NodeConfig, arch string) (preparedRelease, error) {
	group, err := i.serviceGroup()
	if err != nil {
		return preparedRelease{}, err
	}
	releases := filepath.Join(i.Paths.Base, "releases")
	for _, path := range []string{i.Paths.Base, releases} {
		if err := groupDir(path, group); err != nil {
			return preparedRelease{}, err
		}
	}
	stage, err := os.MkdirTemp(releases, ".prepare-")
	if err != nil {
		return preparedRelease{}, err
	}
	defer os.RemoveAll(stage)
	if err := i.prepareFiles(data, arch, stage, group); err != nil {
		return preparedRelease{}, err
	}
	if err := groupDir(stage, group); err != nil {
		return preparedRelease{}, err
	}
	return publish(data, stage, releases, group)
}

func publish(data model.NodeConfig, stage, releases string, group int) (preparedRelease, error) {
	if err := release.Permissions(stage, data, os.Geteuid(), group); err != nil {
		return preparedRelease{}, err
	}
	files, err := release.Inventory(stage, data)
	if err != nil {
		return preparedRelease{}, err
	}
	id, err := release.Identity(data.Generation, unitFor(data), files)
	if err != nil {
		return preparedRelease{}, err
	}
	prepared := preparedRelease{path: filepath.Join(releases, id)}
	if _, err := os.Lstat(prepared.path); !os.IsNotExist(err) {
		if err := release.Permissions(prepared.path, data, os.Geteuid(), group); err != nil {
			return preparedRelease{}, err
		}
		existing, verifyErr := release.Inventory(prepared.path, data)
		if err != nil || verifyErr != nil || !reflect.DeepEqual(files, existing) {
			return preparedRelease{}, fmt.Errorf("existing release differs or is inaccessible; inspect before retrying: %s", prepared.path)
		}
		return prepared, nil
	}
	if err := os.Rename(stage, prepared.path); err != nil {
		return preparedRelease{}, err
	}
	prepared.created = true
	return prepared, safeio.SyncDir(releases)
}

func (i *Installer) prepareFiles(data model.NodeConfig, arch, stage string, group int) error {
	if data.SchemaVersion == 2 {
		return release.Prepare(i.Context, data, arch, stage, filepath.Join(i.Paths.Base, "cache"), group, i.command)
	}
	binary, err := i.binary(data.Runtime.Assets[arch], stage)
	if err != nil {
		return err
	}
	config, err := safeio.Encode(data.Xray)
	if err != nil {
		return err
	}
	path := filepath.Join(stage, "config.json")
	if err := safeio.Atomic(path, config, 0640); err != nil {
		return err
	}
	if err := os.Chown(path, 0, group); err != nil {
		return err
	}
	_, err = i.command(binary, "run", "-test", "-config", path)
	return err
}

func (i *Installer) binary(asset model.Asset, directory string) (string, error) {
	contents, err := artifact.Cached(i.Context, asset, filepath.Join(i.Paths.Base, "cache"))
	if err != nil {
		return "", err
	}
	path, err := artifact.Extract(contents, directory)
	if err != nil {
		return "", err
	}
	version, err := i.command(path, "version")
	if err != nil || !bytes.HasPrefix(version, []byte("Xray "+xray.Version+" ")) {
		return "", fmt.Errorf("unexpected runtime version")
	}
	return path, nil
}
