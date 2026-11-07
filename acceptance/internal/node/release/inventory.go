package release

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

func names(data model.NodeConfig) []string {
	if data.SchemaVersion != 2 {
		return []string{"xray", "config.json"}
	}
	result := []string{"nhop-node", "processes.json"}
	for _, program := range data.Programs {
		result = append(result, program.Name, program.Name+".json")
		if program.Name == "sing-box" {
			result = append(result, "libcronet.so")
		}
	}
	return result
}

// Inventory accepts only the complete set of regular files owned by this release.
func Inventory(directory string, data model.NodeConfig) (map[string]string, error) {
	return inventory(directory, names(data))
}

func inventory(directory string, expected []string) (map[string]string, error) {
	if err := safeio.NoSymlink(directory); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	if len(entries) != len(expected) {
		return nil, fmt.Errorf("release has missing or unexpected files: %s", directory)
	}
	hashes := map[string]string{}
	for _, name := range expected {
		hash, err := safeio.FileHash(filepath.Join(directory, name))
		if err != nil {
			return nil, err
		}
		hashes[name] = hash
	}
	return hashes, nil
}

// Identity separates desired configuration from executable and service revisions.
func Identity(generation, unit string, files map[string]string) (string, error) {
	return safeio.Digest(struct {
		Schema     int               `json:"schema"`
		Generation string            `json:"generation"`
		Unit       string            `json:"unit"`
		Files      map[string]string `json:"files"`
	}{1, generation, unit, files})
}

func SupervisorHash() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return safeio.FileHash(executable)
}
