package safeio

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func NoSymlink(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink: %s", path)
	}
	return nil
}

func PrivateDir(path string) error {
	if err := NoSymlink(path); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return privateDirectory(path)
}

func Read(path string, limit int64) ([]byte, error) {
	if limit < 0 || limit == 1<<63-1 {
		return nil, fmt.Errorf("invalid file size limit")
	}
	file, err := openRegular(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file exceeds size limit: %s", path)
	}
	return data, err
}

func SyncDir(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func Atomic(path string, contents []byte, mode os.FileMode) error {
	if err := NoSymlink(path); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".nhop-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = writeFile(file, contents, mode); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return err
	}
	return SyncDir(filepath.Dir(path))
}

// Create publishes a complete new file without replacing an existing directory
// entry, including a symlink or a concurrently created regular file.
func Create(path string, contents []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".nhop-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := writeFile(file, contents, mode); err != nil {
		return err
	}
	if err := os.Link(file.Name(), path); err != nil {
		return err
	}
	return SyncDir(filepath.Dir(path))
}

func writeFile(file *os.File, contents []byte, mode os.FileMode) error {
	defer file.Close()
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}
