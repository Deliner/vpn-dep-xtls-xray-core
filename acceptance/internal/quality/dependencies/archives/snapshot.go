package archives

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"vpn-script/internal/quality/dependencies/commands"
)

func snapshot(ctx context.Context, run commands.Runner, root string) (map[string]fileIdentity, error) {
	raw, err := run(ctx, "git", "-C", root, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	files := map[string]fileIdentity{}
	for _, entry := range strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00") {
		name, mode, object, err := sourceEntry(entry)
		if err != nil {
			return nil, err
		}
		identity, err := sourceIdentity(root, name, mode, object)
		if err != nil {
			return nil, err
		}
		files[name] = identity
	}
	return files, validateSourceLinks(files)
}

func sourceEntry(entry string) (string, string, string, error) {
	header, name, found := strings.Cut(entry, "\t")
	fields := strings.Fields(header)
	if !found || len(fields) != 3 || fields[2] != "0" || !filepath.IsLocal(name) || filepath.ToSlash(filepath.Clean(name)) != name {
		return "", "", "", fmt.Errorf("unexpected tracked source entry %q", entry)
	}
	if fields[0] != "160000" && fields[0] != "120000" && fields[0] != "100644" && fields[0] != "100755" {
		return "", "", "", fmt.Errorf("source requires tracked regular files, symlinks or Git links: %s", name)
	}
	object, err := hex.DecodeString(fields[1])
	if err != nil || len(object) != 20 {
		return "", "", "", fmt.Errorf("source index requires a full Git object ID: %s", name)
	}
	return name, fields[0], fields[1], nil
}

func submodules(files map[string]fileIdentity) map[string]string {
	result := map[string]string{}
	for name, identity := range files {
		if identity.Gitlink != "" {
			result[name] = identity.Gitlink
		}
	}
	return result
}
