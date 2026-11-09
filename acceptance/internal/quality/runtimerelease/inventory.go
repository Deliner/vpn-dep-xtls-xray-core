package runtimerelease

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"vpn-script/internal/platform/safeio"
)

func readRegular(root, name string, limit int64) ([]byte, error) {
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		return nil, fmt.Errorf("unsafe inventory path: %q", name)
	}
	path := root
	for _, part := range strings.Split(name, "/") {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink in inventory path: %s", name)
		}
	}
	return safeio.Read(path, limit)
}

func hash(raw []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func inventory(root, name, expected string) (map[string]string, error) {
	raw, err := readRegular(root, name, 4_000_000)
	if err != nil {
		return nil, err
	}
	if hash(raw) != expected {
		return nil, fmt.Errorf("inventory differs from reviewed native CI: %s", name)
	}
	entries := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		sha, path, ok := strings.Cut(line, "  ")
		path = strings.TrimPrefix(path, "./")
		if !ok || !digest(sha, 32) || entries[path] != "" {
			return nil, fmt.Errorf("invalid or duplicate inventory entry: %s", name)
		}
		body, err := readRegular(root, path, 150_000_000)
		if err != nil {
			return nil, err
		}
		if hash(body) != sha {
			return nil, fmt.Errorf("inventory content changed: %s", path)
		}
		entries[path] = sha
	}
	return entries, nil
}

func completeTree(root, name string, entries map[string]string) error {
	return filepath.WalkDir(filepath.Join(root, name), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() || entries[filepath.ToSlash(relative)] == "" {
			return fmt.Errorf("unrecorded or special candidate input: %s", relative)
		}
		return nil
	})
}
