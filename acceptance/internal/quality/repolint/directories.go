package repolint

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"vpn-script/internal/quality/diagnostics"
	"vpn-script/internal/quality/repository"
)

const MaxDirectoryEntries = 12

func directorySize(root, path string) ([]diagnostics.Finding, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !repository.Excluded(root, filepath.Join(path, entry.Name())) {
			names = append(names, entry.Name())
		}
	}
	if len(names) <= MaxDirectoryEntries {
		return nil, nil
	}
	return []diagnostics.Finding{diagnostics.Issue(diagnostics.DirectorySize, "%s: %d immediate entries (maximum 12): %s", path, len(names), strings.Join(names, ", "))}, nil
}

func Directories(root string) ([]diagnostics.Finding, error) {
	var findings []diagnostics.Finding
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if repository.Excluded(root, path) && path != root {
			return filepath.SkipDir
		}
		items, err := directorySize(root, path)
		findings = append(findings, items...)
		return err
	})
	return findings, err
}
