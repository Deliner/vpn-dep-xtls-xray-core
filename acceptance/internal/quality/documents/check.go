package documents

import (
	"path/filepath"
	"strings"

	"vpn-script/internal/quality/diagnostics"
	"vpn-script/internal/quality/repository"
)

func archivedSnapshot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return strings.HasPrefix(filepath.ToSlash(relative), Research+"/00-control/archive/RESEARCH-")
}

func Check(root string) ([]diagnostics.Finding, error) {
	paths, err := repository.Files(root, ".md")
	if err != nil {
		return nil, err
	}
	var findings []diagnostics.Finding
	for _, path := range paths {
		if archivedSnapshot(root, path) {
			continue
		}
		items, err := Links(path)
		if err != nil {
			return findings, err
		}
		for _, item := range items {
			findings = append(findings, diagnostics.Issue(diagnostics.DocumentationLinks, "%s", item))
		}
	}
	if err := CheckInventory(root); err != nil {
		findings = append(findings, diagnostics.Issue(diagnostics.ResearchInventory, "%v", err))
	}
	return findings, nil
}
