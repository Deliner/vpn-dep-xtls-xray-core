package owned

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^vpn-dep-[a-z0-9._-]+$`)
var partPattern = regexp.MustCompile(`^[A-Za-z0-9_+.-]+$`)
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var sumPattern = regexp.MustCompile(`^h1:[A-Za-z0-9+/]{43}=$`)

func requestIdentity(module, revision string) (string, error) {
	parts := strings.Split(module, "/")
	if len(parts) < 2 || parts[0] != "Deliner" || !namePattern.MatchString(parts[1]) || !commitPattern.MatchString(revision) {
		return "", fmt.Errorf("expected Deliner/vpn-dep-repository[/submodule] and full commit SHA")
	}
	for _, part := range parts[2:] {
		if part == "." || part == ".." || !partPattern.MatchString(part) {
			return "", fmt.Errorf("invalid owned module suffix")
		}
	}
	return strings.Join(parts[:2], "/"), nil
}

func verifyIdentity(result Module, module, repository, revision string) error {
	if result.Error != "" {
		return fmt.Errorf("owned module download: %s", result.Error)
	}
	if result.Path != "github.com/"+module || result.Version == "" || !sumPattern.MatchString(result.Sum) || !sumPattern.MatchString(result.GoModSum) {
		return fmt.Errorf("incomplete owned module identity")
	}
	if result.Origin == nil || result.Origin.VCS != "git" || result.Origin.Hash != revision || strings.TrimSuffix(result.Origin.URL, ".git") != "https://github.com/"+repository {
		return fmt.Errorf("owned module origin differs from reviewed fork/commit")
	}
	for _, path := range []string{result.Info, result.GoMod, result.Zip, result.Dir} {
		if err := cachePath(path); err != nil {
			return err
		}
	}
	return nil
}

func cachePath(path string) error {
	cache := os.Getenv("GOMODCACHE")
	if !filepath.IsAbs(cache) || !filepath.IsAbs(path) {
		return fmt.Errorf("module artifacts require explicit absolute GOMODCACHE paths")
	}
	relative, err := filepath.Rel(cache, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("module artifact escapes the configured cache")
	}
	return nil
}
