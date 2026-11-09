package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"vpn-script/internal/platform/safeio"
)

// VerifyPackage is suitable for independently downloaded release assets.
func VerifyPackage(policy Policy, directory string) error {
	if err := policy.valid(); err != nil {
		return err
	}
	if err := stagePolicy(policy, directory); err != nil {
		return err
	}
	for _, arch := range []string{"amd64", "arm64"} {
		raw, err := stageRead(directory, archiveName(arch), 150_000_000)
		if err != nil {
			return err
		}
		if err := verifyArchive(policy, arch, raw); err != nil {
			return err
		}
	}
	sums, err := stageSums(directory)
	if err != nil {
		return err
	}
	raw, err := stageRead(directory, "SHA256SUMS", 4096)
	if err != nil {
		return err
	}
	if string(raw) != sums {
		return fmt.Errorf("Cronet package checksums differ")
	}
	return nil
}

func stageRead(directory, name string, limit int64) ([]byte, error) {
	path, err := regular(directory, name)
	if err != nil {
		return nil, err
	}
	return safeio.Read(path, limit)
}

func stageSums(directory string) (string, error) {
	var sums string
	for _, name := range []string{archiveName("amd64"), archiveName("arm64"), "policy.json"} {
		path, err := regular(directory, name)
		if err != nil {
			return "", err
		}
		sha, err := safeio.FileHash(filepath.Clean(path))
		if err != nil {
			return "", err
		}
		sums += sha + "  " + name + "\n"
	}
	return sums, nil
}

func stagePolicy(policy Policy, directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	if len(entries) != 4 {
		return fmt.Errorf("expected exactly two Cronet archives, policy and checksums")
	}
	want, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return err
	}
	actual, err := stageRead(directory, "policy.json", 32768)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, append(want, '\n')) {
		return fmt.Errorf("package policy differs from reviewed policy")
	}
	return nil
}
