package release

import (
	"fmt"
	"vpn-script/internal/platform/safeio"
)

// ExtractLibrary publishes only a fully reviewed archive's library. It never
// extracts archive paths or overwrites an existing destination.
func ExtractLibrary(policy Policy, arch, archive, output string) error {
	if err := policy.valid(); err != nil {
		return err
	}
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unsupported Cronet CPU")
	}
	raw, err := safeio.Read(archive, 150_000_000)
	if err != nil {
		return err
	}
	if err := verifyArchive(policy, arch, raw); err != nil {
		return err
	}
	members, err := readArchive(raw)
	if err != nil {
		return err
	}
	return safeio.Create(output, members["libcronet.so"], 0755)
}
