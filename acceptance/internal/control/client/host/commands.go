package host

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func missing(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 2
}

// Commands verifies the executables resolved by the controller before it takes
// ownership. A writable PATH component cannot substitute a privileged tool.
func Commands() error {
	for _, variable := range []string{"LD_PRELOAD", "LD_LIBRARY_PATH"} {
		if os.Getenv(variable) != "" {
			return fmt.Errorf("client controller requires an unset %s", variable)
		}
	}
	for _, name := range []string{"getent", "useradd", "id", "ip", "nft", "resolvectl"} {
		path, err := exec.LookPath(name)
		if err != nil {
			return err
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		if err := protectedParents(filepath.Dir(path), 0); err != nil {
			return err
		}
		if err := protectedFile(path, 0); err != nil {
			return err
		}
	}
	return nil
}
