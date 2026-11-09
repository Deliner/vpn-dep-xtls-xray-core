package node

import (
	"bufio"
	"bytes"
	"debug/elf"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func readJSON(path string, target any) error {
	raw, err := safeio.Read(path, 10_000_000)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func manifest(path string) (map[string]string, error) {
	raw, err := safeio.Read(path, 10_000_000)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		sha, name, ok := strings.Cut(scanner.Text(), "  ")
		name = strings.TrimPrefix(name, "./")
		if !ok || !validate.Hash(sha) || !filepath.IsLocal(name) || filepath.Clean(name) != name || result[name] != "" {
			return nil, fmt.Errorf("invalid checksum manifest entry: %s", path)
		}
		result[name] = sha
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty checksum manifest: %s", path)
	}
	return result, nil
}

func verifyFiles(root string, files map[string]string) error {
	for name, expected := range files {
		actual, err := safeio.FileHash(filepath.Join(root, name))
		if err != nil || actual != expected {
			return fmt.Errorf("changed evidence file: %s", name)
		}
	}
	return nil
}

func completeInventory(root string) error {
	files, err := manifest(filepath.Join(root, "evidence.sha256"))
	if err != nil {
		return err
	}
	files["evidence.sha256"], err = safeio.FileHash(filepath.Join(root, "evidence.sha256"))
	if err != nil {
		return err
	}
	return verifyTree(root, files)
}

func verifyTree(root string, files map[string]string) error {
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
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
		if files[name] == "" {
			return fmt.Errorf("unrecorded evidence file: %s", name)
		}
		seen[name] = true
		return nil
	})
	if err != nil {
		return err
	}
	if len(seen) != len(files) {
		return fmt.Errorf("incomplete evidence inventory")
	}
	return verifyFiles(root, files)
}

func equalFiles(first, second string) error {
	a, err := safeio.FileHash(first)
	if err != nil {
		return err
	}
	b, err := safeio.FileHash(second)
	if err != nil || a != b {
		return fmt.Errorf("files differ: %s and %s", first, second)
	}
	return nil
}

func textFile(path, expected string) error {
	raw, err := safeio.Read(path, 1<<20)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(raw)) != expected {
		return fmt.Errorf("unexpected evidence: %s", path)
	}
	return nil
}

func linuxELF(path, arch string) error {
	file, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	want := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}[arch]
	if file.Class != elf.ELFCLASS64 || file.Data != elf.ELFDATA2LSB || file.Machine != want {
		return fmt.Errorf("wrong native ELF target: %s", path)
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
			return nil, fmt.Errorf("invalid identity file: %s", path)
		}
		result[key] = value
	}
	return result, nil
}
