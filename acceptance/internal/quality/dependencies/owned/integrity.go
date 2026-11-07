package owned

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/mod/sumdb/dirhash"
)

// Rehash both cache representations instead of trusting cached .ziphash files.
// Sum and GoModSum have already been authenticated by the configured Go downloader.
func verifyContent(module Module) error {
	zipSum, err := dirhash.HashZip(module.Zip, dirhash.Hash1)
	if err != nil {
		return err
	}
	if zipSum != module.Sum {
		return fmt.Errorf("owned module ZIP differs from authenticated checksum")
	}
	directorySum, err := dirhash.HashDir(module.Dir, module.Path+"@"+module.Version, dirhash.Hash1)
	if err != nil {
		return err
	}
	if directorySum != module.Sum {
		return fmt.Errorf("owned module extracted source differs from authenticated checksum")
	}
	modSum, err := dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) { return os.Open(module.GoMod) })
	if err != nil {
		return err
	}
	if modSum != module.GoModSum {
		return fmt.Errorf("owned module go.mod differs from authenticated checksum")
	}
	return nil
}
