package controller

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/safeio"
)

// pruneStopped removes only verified executable snapshots of the previous
// explicitly stopped session. Its journal and private diagnostics remain valid.
func pruneStopped(directory string) error {
	if directory == "" {
		return nil
	}
	if err := snapshotMembers(directory); err != nil {
		return err
	}
	var request session.Request
	if err := host.ReadSession(directory, "request.json", &request); err != nil {
		return err
	}
	var paths []string
	for name, artifact := range map[string]session.Artifact{"native": request.Native, "frontend": request.Frontend, "libcronet.so": request.Cronet} {
		path := filepath.Join(directory, name)
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			continue
		}
		if err := safeio.CheckAccess(path, 0755, 0, 0); err != nil {
			return err
		}
		if digest, err := safeio.FileHash(path); err != nil || digest != artifact.SHA256 {
			return fmt.Errorf("stopped runtime snapshot changed; refusing cleanup")
		}
		paths = append(paths, path)
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return safeio.SyncDir(directory)
}

func snapshotMembers(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "native", "frontend", "libcronet.so", "native.json", "frontend.json", "native.log", "frontend.log", "request.json", "status.json", "session.lock":
		default:
			return fmt.Errorf("unexpected member of stopped client snapshot; refusing cleanup")
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular member of stopped client snapshot; refusing cleanup")
		}
	}
	return nil
}
