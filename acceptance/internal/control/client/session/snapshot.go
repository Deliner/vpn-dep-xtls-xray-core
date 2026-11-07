package session

import (
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
		return nil, err
	}
	return lock, nil
}

func snapshot(request Request) error {
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
