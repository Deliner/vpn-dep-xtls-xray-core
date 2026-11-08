package archives

import (
	"archive/tar"
	"fmt"
	"path"
	"strings"
)

// Only a direct link to a tracked regular file is part of the source contract.
// Directory links, chains, cycles, external targets and Gitlink contents are not.
func validateSourceLinks(files map[string]fileIdentity) error {
	for name, identity := range files {
		if identity.Symlink == "" {
			continue
		}
		target := identity.Symlink
		if len(target) > 4096 || path.IsAbs(target) || strings.ContainsAny(target, "\x00\r\n") {
			return fmt.Errorf("unsafe source symlink: %s", name)
		}
		resolved := path.Join(path.Dir(name), target)
		destination, found := files[resolved]
		if !found || destination.SHA256 == "" || destination.Symlink != "" || destination.Gitlink != "" {
			return fmt.Errorf("source symlink must name a tracked regular file: %s -> %s", name, target)
		}
	}
	return nil
}

func verifySourceLink(header *tar.Header, identity fileIdentity) error {
	if header.Typeflag != tar.TypeSymlink || header.Size != 0 || header.Linkname != identity.Symlink {
		return fmt.Errorf("archive symlink differs from pinned checkout: %s", header.Name)
	}
	return nil
}

func sourceLinks(files map[string]fileIdentity) map[string]string {
	result := map[string]string{}
	for name, identity := range files {
		if identity.Symlink != "" {
			result[name] = identity.Symlink
		}
	}
	return result
}
