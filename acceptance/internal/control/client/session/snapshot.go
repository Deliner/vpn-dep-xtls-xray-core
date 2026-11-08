package session

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

// prepare snapshots bytes before any engine execution. Hashes must come from a
// trusted release/candidate record; they do not authenticate arbitrary downloads.
func prepare(request Request) (*os.File, error) {
	if err := network.Validate(request.Profile, request.Network); err != nil {
		return nil, err
	}
	if request.EngineGID < 100 || int64(request.EngineGID) > 4294967294 || !filepath.IsAbs(request.Directory) {
		return nil, fmt.Errorf("invalid dedicated engine GID or absolute session directory")
	}
	if err := os.Mkdir(request.Directory, 0711); err != nil {
		return nil, fmt.Errorf("session requires a new directory in a protected parent: %w", err)
	}
	if err := os.Chmod(request.Directory, 0711); err != nil {
		return nil, err
	}
	lock, err := safeio.Lock(filepath.Join(request.Directory, "session.lock"))
	if err != nil {
		return nil, err
	}
	if err := snapshot(request); err != nil {
		lock.Close()
		// This newly created directory has never been executed or exposed writable
		// to an engine. A failed snapshot must not accumulate copied runtimes.
		return nil, errors.Join(err, os.RemoveAll(request.Directory))
	}
	return lock, nil
}

func snapshot(request Request) error {
	if err := safeio.NewJSON(filepath.Join(request.Directory, "request.json"), request); err != nil {
		return err
	}
	artifacts := map[string]Artifact{"native": request.Native, "frontend": request.Frontend, "libcronet.so": request.Cronet}
	for name, artifact := range artifacts {
		if err := copyArtifact(request.Directory, name, artifact); err != nil {
			return err
		}
	}
	tun, err := network.ManualTUN(request.Profile, request.Network)
	if err != nil {
		return err
	}
	if err := nativeConfig(request); err != nil {
		return err
	}
	return safeio.NewJSON(filepath.Join(request.Directory, "frontend.json"), tun)
}

func nativeConfig(request Request) error {
	path := filepath.Join(request.Directory, "native.json")
	if err := safeio.NewJSON(path, request.Profile.Config); err != nil {
		return err
	}
	// Xray requires a filename suffix even with its explicit JSON format option.
	// Root retains ownership; only the dedicated engine group receives read access.
	if err := os.Chown(path, 0, request.EngineGID); err != nil {
		return err
	}
	return os.Chmod(path, 0640)
}

func copyArtifact(directory, name string, artifact Artifact) error {
	if !validate.Hash(artifact.SHA256) {
		return fmt.Errorf("%s requires an explicit trusted runtime SHA-256", name)
	}
	data, err := safeio.Read(artifact.Path, 256<<20)
	if err != nil {
		return fmt.Errorf("read %s runtime: %w", name, err)
	}
	if safeio.Hash(data) != artifact.SHA256 {
		return fmt.Errorf("%s runtime hash mismatch; nothing executed", name)
	}
	return safeio.Create(filepath.Join(directory, name), data, 0755)
}

func verifySnapshot(request Request) error {
	if err := safeio.CheckAccess(request.Directory, os.ModeDir|0711, os.Geteuid(), os.Getegid()); err != nil {
		return err
	}
	for name, artifact := range map[string]Artifact{"native": request.Native, "frontend": request.Frontend, "libcronet.so": request.Cronet} {
		path := filepath.Join(request.Directory, name)
		if err := safeio.CheckAccess(path, 0755, os.Geteuid(), os.Getegid()); err != nil {
			return err
		}
		if digest, err := safeio.FileHash(path); err != nil || digest != artifact.SHA256 {
			return fmt.Errorf("%s snapshot hash changed; reconnect refused", name)
		}
	}
	tun, err := network.ManualTUN(request.Profile, request.Network)
	if err != nil {
		return err
	}
	for name, value := range map[string]any{"request.json": request, "native.json": request.Profile.Config, "frontend.json": tun} {
		mode, gid := os.FileMode(0600), os.Getegid()
		if name == "native.json" {
			mode, gid = 0640, request.EngineGID
		}
		if err := verifyConfiguration(request.Directory, name, value, mode, gid); err != nil {
			return err
		}
	}
	return nil
}

func verifyConfiguration(directory, name string, value any, mode os.FileMode, gid int) error {
	path := filepath.Join(directory, name)
	if err := safeio.CheckAccess(path, mode, os.Geteuid(), gid); err != nil {
		return err
	}
	expected, err := safeio.FormattedJSON(value)
	if err != nil {
		return err
	}
	actual, err := safeio.Read(path, 10_000_000)
	if err != nil || !bytes.Equal(actual, expected) {
		return fmt.Errorf("%s snapshot changed; reconnect refused", name)
	}
	return nil
}
