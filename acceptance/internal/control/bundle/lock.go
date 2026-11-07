package bundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"vpn-script/internal/platform/safeio"
)

// Lock gives one render, deploy or verify operation ownership of this bundle.
// Keep the returned descriptor open until all dependent reads and writes finish.
func Lock(directory string) (*os.File, error) {
	lock, err := safeio.Lock(filepath.Join(directory, ".render.lock"))
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil, fmt.Errorf("bundle is busy: wait for the active render/deploy/verify operation in %s: %w", directory, err)
	}
	if err != nil {
		return nil, fmt.Errorf("lock bundle %s: %w", directory, err)
	}
	return lock, nil
}
