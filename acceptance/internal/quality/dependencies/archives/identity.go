package archives

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

func sourceIdentity(root, name, mode, object string) (fileIdentity, error) {
	if mode == "160000" {
		return fileIdentity{Gitlink: object}, nil
	}
	file := filepath.Join(root, name)
	info, err := os.Lstat(file)
	if err != nil {
		return fileIdentity{}, err
	}
	if mode == "120000" {
		if info.Mode()&os.ModeSymlink == 0 {
			return fileIdentity{}, fmt.Errorf("tracked symlink was replaced: %s", name)
		}
		target, err := os.Readlink(file)
		if err != nil || target == "" {
			return fileIdentity{}, fmt.Errorf("cannot read source symlink %s: %v", name, err)
		}
		return fileIdentity{Symlink: target}, nil
	}
	if !info.Mode().IsRegular() || (info.Mode()&0111 != 0) != (mode == "100755") {
		return fileIdentity{}, fmt.Errorf("source type or executable bits differ from Git: %s", name)
	}
	digest, err := safeio.FileHash(file)
	return fileIdentity{SHA256: digest, Executable: info.Mode()&0111 != 0}, err
}
