package netem

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func readJSON(path string, target any) error {
	raw, err := safeio.Read(path, 16<<20)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func readManifest(path string) (map[string]string, error) {
	raw, err := safeio.Read(path, 16<<20)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		sha, name, ok := strings.Cut(scanner.Text(), "  ")
		name = strings.TrimPrefix(name, "./")
		if !ok || !validate.Hash(sha) || !filepath.IsLocal(name) || filepath.Clean(name) != name || result[name] != "" {
			return nil, fmt.Errorf("invalid manifest entry in %s", path)
		}
		result[name] = sha
	}
	if err := scanner.Err(); err != nil || len(result) == 0 {
		return nil, fmt.Errorf("empty or incomplete manifest: %s: %v", path, err)
	}
	return result, nil
}

func completeInventory(root, name string) (map[string]string, error) {
	files, err := readManifest(filepath.Join(root, name))
	if err != nil {
		return nil, err
	}
	observed, err := regularFiles(root)
	if err != nil {
		return nil, err
	}
	delete(observed, name)
	if len(files) != len(observed) {
		return nil, fmt.Errorf("incomplete evidence inventory: %s", root)
	}
	for path := range observed {
		actual, err := safeio.FileHash(filepath.Join(root, path))
		if err != nil || files[path] != actual {
			return nil, fmt.Errorf("unrecorded or changed evidence: %s: %v", path, err)
		}
	}
	return files, nil
}

func regularFiles(root string) (map[string]bool, error) {
	files := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("special evidence file: %s", path)
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[name] = true
		return nil
	})
	return files, err
}

func equalFiles(first, second string) error {
	a, err := safeio.FileHash(first)
	if err != nil {
		return err
	}
	b, err := safeio.FileHash(second)
	if err != nil || a != b {
		return fmt.Errorf("different evidence: %s and %s", first, second)
	}
	return nil
}

func textFile(path, expected string) error {
	raw, err := safeio.Read(path, 1<<20)
	if err != nil || string(raw) != expected {
		return fmt.Errorf("unexpected evidence text: %s: %v", path, err)
	}
	return nil
}

func properties(path string) (map[string]string, error) {
	raw, err := safeio.Read(path, 1<<20)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" || value == "" || result[key] != "" {
			return nil, fmt.Errorf("invalid identity: %s", path)
		}
		result[key] = value
	}
	return result, nil
}
