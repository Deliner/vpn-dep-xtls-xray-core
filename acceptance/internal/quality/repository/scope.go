package repository

import (
	"io/fs"
	"path/filepath"
	"strings"
)

func Excluded(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	name := filepath.Base(path)
	return parts[0] == ".git" || parts[0] == "build" || name == ".DS_Store" || strings.HasPrefix(name, "._")
}

func Files(root, suffix string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if Excluded(root, path) && path != root {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && strings.HasSuffix(path, suffix) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
