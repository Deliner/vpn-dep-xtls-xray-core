package install

import (
	"bytes"
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"vpn-script/internal/node/fleet/service"
	"vpn-script/internal/platform/safeio"
)

func linuxRoot() error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 || os.Getegid() != 0 {
		return fmt.Errorf("Fleet service management requires Linux root with primary group root")
	}
	return nil
}

func protectedExecutable(path string) (string, error) {
	if err := safeio.ProtectedParent(path, 0); err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || info.Mode().Perm()&0100 == 0 {
		return "", fmt.Errorf("unsafe executable permissions")
	}
	return path, safeio.CheckAccess(path, info.Mode(), 0, -1)
}

func runningBinary() ([]byte, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if _, err := protectedExecutable(path); err != nil {
		return nil, err
	}
	raw, err := safeio.Read(path, 100_000_000)
	if err != nil {
		return nil, err
	}
	file, err := elf.NewFile(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("Fleet service installer must be a native Linux executable")
	}
	defer file.Close()
	wanted := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}[runtime.GOARCH]
	if wanted == 0 || file.Machine != wanted || file.Class != elf.ELFCLASS64 || file.Data != elf.ELFDATA2LSB {
		return nil, fmt.Errorf("Fleet service executable architecture mismatch")
	}
	return raw, nil
}

func validateStatePath(state string) error {
	if err := service.PermanentPath(state); err != nil {
		return err
	}
	if state == service.Directory || strings.HasPrefix(state, service.Directory+"/") {
		return fmt.Errorf("agent journal cannot be stored in the service installation")
	}
	return safeio.ProtectedParent(state, 0)
}
