package release

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func regular(root, name string) (string, error) {
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		return "", fmt.Errorf("unsafe evidence path: %s", name)
	}
	if err := safeio.NoSymlink(root); err != nil {
		return "", err
	}
	path := root
	for _, part := range strings.Split(name, "/") {
		path = filepath.Join(path, part)
		if err := safeio.NoSymlink(path); err != nil {
			return "", err
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("nonregular evidence: %s", name)
	}
	return path, nil
}

func inventory(root, expected string, manifests map[string]string) (map[string]string, error) {
	path, err := regular(root, "evidence.sha256")
	if err != nil {
		return nil, err
	}
	raw, err := safeio.Read(path, 4_000_000)
	if err != nil {
		return nil, err
	}
	if safeio.Hash(raw) != expected {
		return nil, fmt.Errorf("Cronet evidence differs from reviewed native run")
	}
	entries := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		sha, name, ok := strings.Cut(line, "  ")
		name = strings.TrimPrefix(name, "./")
		if !ok || !validate.Hash(sha) || entries[name] != "" {
			return nil, fmt.Errorf("invalid evidence entry")
		}
		path, err := regular(root, name)
		if err != nil {
			return nil, err
		}
		actual, err := safeio.FileHash(path)
		if err != nil || actual != sha {
			return nil, fmt.Errorf("changed Cronet input: %s", name)
		}
		entries[name] = sha
	}
	return entries, complete(root, entries, manifests)
}

func complete(root string, entries, manifests map[string]string) error {
	if err := verifyManifests(root, manifests); err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.IsDir() {
			return nil
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)
		if !item.Type().IsRegular() {
			return fmt.Errorf("special evidence entry: %s", name)
		}
		if manifests[name] != "" || entries[name] != "" || name == "evidence.sha256" || name == "integrity-verification.log" || name == "naive/integrity-verification.log" {
			return nil
		}
		return fmt.Errorf("unrecorded Cronet evidence: %s", name)
	})
}

func verifyManifests(root string, manifests map[string]string) error {
	for name, sha := range manifests {
		path, err := regular(root, name)
		if err != nil {
			return err
		}
		actual, err := safeio.FileHash(path)
		if err != nil || actual != sha {
			return fmt.Errorf("changed nested manifest: %s", name)
		}
	}
	return nil
}
