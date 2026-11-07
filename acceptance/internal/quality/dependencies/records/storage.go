package records

import (
	"fmt"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

func Save(name string, value any) error {
	path, err := recordPath(name)
	if err != nil {
		return err
	}
	for _, directory := range []string{"build", "build/dependencies"} {
		if err := safeio.PrivateDir(directory); err != nil {
			return err
		}
	}
	return safeio.JSON(path, value)
}

func recordPath(name string) (string, error) {
	if name == "." || name == ".." || name == "" || filepath.Base(name) != name {
		return "", fmt.Errorf("record requires a simple filename")
	}
	return filepath.Join("build/dependencies", name), nil
}
