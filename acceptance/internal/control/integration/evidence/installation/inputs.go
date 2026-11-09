package installation

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

// CaptureInputs preserves independently inspectable engine bytes and the exact
// downloaded archives after the service stops, before the owned fixture is removed.
func CaptureInputs(output string, archives map[string]string, previous map[string]evidence.Runtime) error {
	if err := OwnedHost(); err != nil {
		return err
	}
	sources, err := captureSources(previous)
	if err != nil {
		return err
	}
	for kind, source := range sources {
		destination := filepath.Join(output, kind)
		if err := os.Mkdir(destination, 0700); err != nil {
			return err
		}
		names := []string{"xray", "sing-box", "libcronet.so"}
		if kind == "archives" {
			var err error
			names, err = captureArchiveNames(source, archives)
			if err != nil {
				return err
			}
		}
		for _, name := range names {
			raw, err := safeio.Read(filepath.Join(source, name), 100_000_000)
			if err != nil {
				return err
			}
			if kind == "archives" && safeio.Hash(raw)+".archive" != name {
				return fmt.Errorf("captured archive changed after cache verification")
			}
			if err := safeio.Atomic(filepath.Join(destination, name), raw, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}

func captureSources(previous map[string]evidence.Runtime) (map[string]string, error) {
	sources := map[string]string{"runtimes": base + "/current", "archives": base + "/cache"}
	if len(previous) == 0 {
		return sources, nil
	}
	directory := filepath.Dir(previous["server/xray"].Path)
	if len(previous) != 4 || filepath.Dir(directory) != base+"/releases" || !validate.Hash(filepath.Base(directory)) {
		return nil, fmt.Errorf("historical runtimes are outside the retained release")
	}
	for _, role := range []string{"client/xray", "server/xray", "client/sing-box", "server/sing-box"} {
		value, exists := previous[role]
		if !exists || filepath.Dir(value.Path) != directory || !value.VerifiedUnchanged || value.Error != "" {
			return nil, fmt.Errorf("historical runtime interval is incomplete")
		}
	}
	if err := safeio.NoSymlink(directory); err != nil {
		return nil, err
	}
	sources["previous-runtimes"] = directory
	return sources, nil
}

func captureArchiveNames(directory string, expected map[string]string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil || len(expected) == 0 || len(entries) != len(expected) {
		return nil, fmt.Errorf("installed archive cache is incomplete")
	}
	names, allowed := []string{}, map[string]bool{}
	for _, sha := range expected {
		if !validate.Hash(sha) || allowed[sha+".archive"] {
			return nil, fmt.Errorf("invalid or duplicate captured archive identity")
		}
		allowed[sha+".archive"] = true
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !allowed[entry.Name()] {
			return nil, fmt.Errorf("unreviewed or linked captured archive")
		}
		names = append(names, entry.Name())
	}
	return names, nil
}
