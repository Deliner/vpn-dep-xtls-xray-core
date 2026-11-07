package archives

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vpn-script/internal/platform/safeio"
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
		if mode == "160000" {
			files[name] = fileIdentity{Gitlink: object}
			continue
		}
		path := filepath.Join(root, name)
		digest, err := safeio.FileHash(path)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("source must be regular: %s: %v", name, err)
		}
		files[name] = fileIdentity{SHA256: digest, Executable: info.Mode()&0111 != 0}
	}
	return files, nil
}

func sourceEntry(entry string) (string, string, string, error) {
	header, name, found := strings.Cut(entry, "\t")
	fields := strings.Fields(header)
	if !found || len(fields) != 3 || fields[2] != "0" || !filepath.IsLocal(name) || filepath.ToSlash(filepath.Clean(name)) != name {
		return "", "", "", fmt.Errorf("unexpected tracked source entry %q", entry)
	}
	if fields[0] != "160000" && fields[0] != "100644" && fields[0] != "100755" {
		return "", "", "", fmt.Errorf("source requires regular files or explicit Git links: %s", name)
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
