package repository

import (
	"path/filepath"
	"slices"
	"strings"
)

func Packages(root string) ([]string, error) {
	files, err := Files(root, ".go")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var patterns []string
	for _, file := range files {
		if strings.HasPrefix(filepath.Base(file), ".") || strings.HasPrefix(filepath.Base(file), "_") {
			continue
		}
		relative, err := filepath.Rel(root, filepath.Dir(file))
		if err != nil {
			return nil, err
		}
		pattern := "./" + filepath.ToSlash(relative)
		if !seen[pattern] {
			patterns = append(patterns, pattern)
			seen[pattern] = true
		}
	}
	slices.Sort(patterns)
	return patterns, nil
}
