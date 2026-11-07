package archives

import (
	"archive/tar"
	"fmt"
	"strings"
)

func verifyArchiveComment(header *tar.Header, prefix string) error {
	root := strings.TrimSuffix(prefix, "/")
	if header.Name != "pax_global_header" || len(root) < 41 || len(header.PAXRecords) != 1 || header.PAXRecords["comment"] != root[len(root)-40:] {
		return fmt.Errorf("archive global metadata does not identify the selected Git commit")
	}
	return nil
}

func verifyDirectory(name, prefix string, expected map[string]fileIdentity, seen map[string]bool) error {
	name = strings.TrimSuffix(name, "/")
	if name == strings.TrimSuffix(prefix, "/") {
		return nil
	}
	if !strings.HasPrefix(name, prefix) {
		return fmt.Errorf("unexpected archive directory %q", name)
	}
	name = strings.TrimPrefix(name, prefix)
	if identity, found := expected[name]; found && identity.Gitlink != "" {
		if seen[name] {
			return fmt.Errorf("duplicate Git submodule directory %s", name)
		}
		seen[name] = true
		return nil
	}
	for file := range expected {
		if strings.HasPrefix(file, name+"/") {
			return nil
		}
	}
	return fmt.Errorf("directory has no counterpart in the selected source tree: %s", name)
}
