//go:build darwin || linux

package safeio

import (
	"os"
	"syscall"
)

func Lock(path string) (*os.File, error) {
	file, err := openRegular(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}
