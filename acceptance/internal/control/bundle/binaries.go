package bundle

import (
	"debug/elf"
	"fmt"
	"path/filepath"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

var installerFiles = map[string]string{"x86_64": "nhop-node-linux-amd64", "aarch64": "nhop-node-linux-arm64"}

func validBinary(path, arch string) error {
	file, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("expected a built Linux installer: %s", path)
	}
	defer file.Close()
	expected := elf.EM_X86_64
	if arch == "aarch64" {
		expected = elf.EM_AARCH64
	}
	if file.Machine != expected || file.Class != elf.ELFCLASS64 {
		return fmt.Errorf("wrong installer architecture: %s", path)
	}
	return nil
}

func copyInstallers(source, directory string) (map[string]model.Binary, error) {
	result := map[string]model.Binary{}
	for arch, filename := range installerFiles {
		path := filepath.Join(source, filename)
		if err := validBinary(path, arch); err != nil {
			return nil, err
		}
		contents, err := safeio.Read(path, 100_000_000)
		if err != nil {
			return nil, err
		}
		relative := "installers/" + filename
		if err := safeio.Atomic(filepath.Join(directory, relative), contents, 0700); err != nil {
			return nil, err
		}
		result[arch] = model.Binary{File: relative, SHA256: safeio.Hash(contents)}
	}
	return result, nil
}
