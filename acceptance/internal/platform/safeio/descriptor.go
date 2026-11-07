//go:build darwin || linux

package safeio

import (
	"fmt"
	"os"
	"syscall"
)

// Open and validate the same object. A path check followed by os.Open would
// permit replacing the final component with a symlink between the two calls.
func openNoFollow(path string, flags int, mode os.FileMode) (*os.File, error) {
	fd, err := syscall.Open(path, flags|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, uint32(mode.Perm()))
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}

func openRegular(path string, flags int, mode os.FileMode) (*os.File, error) {
	file, err := openNoFollow(path, flags, mode)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("expected regular file: %s", path)
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func privateDirectory(path string) error {
	file, err := openNoFollow(path, os.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Chmod(0700)
}
