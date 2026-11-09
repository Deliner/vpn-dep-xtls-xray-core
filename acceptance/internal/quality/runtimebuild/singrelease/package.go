package singrelease

import (
	"fmt"
	"os"
	"path/filepath"
	"vpn-script/internal/platform/safeio"
)

// Package creates an installer-compatible archive with the complete tested Go
// source tree. Cronet's full source/notice release remains an additional input
// to publication; creating this directory does not publish a complete release.
func Package(policy Policy, input Inputs, output string) (err error) {
	entries, err := Verify(policy, input)
	if err != nil {
		return err
	}
	if err := os.Mkdir(output, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(output)
		}
	}()
	name := archiveName(policy, input.Arch)
	if err := writeArchive(policy, input, entries, filepath.Join(output, name)); err != nil {
		return err
	}
	if err := VerifyArchive(policy, input.Arch, filepath.Join(output, name)); err != nil {
		return err
	}
	sha, err := safeio.FileHash(filepath.Join(output, name))
	if err != nil {
		return err
	}
	return safeio.Create(filepath.Join(output, "SHA256SUMS"), []byte(fmt.Sprintf("%s  %s\n", sha, name)), 0600)
}
