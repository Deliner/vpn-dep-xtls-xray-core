package installation

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

// CaptureInputs preserves independently inspectable engine bytes and the exact
// downloaded archives after the service stops, before the owned fixture is removed.
func CaptureInputs(output string) error {
	if err := OwnedHost(); err != nil {
		return err
	}
	for kind, source := range map[string]string{"runtimes": base + "/current", "archives": base + "/cache"} {
		destination := filepath.Join(output, kind)
		if err := os.Mkdir(destination, 0700); err != nil {
			return err
		}
		names := []string{"xray", "sing-box", "libcronet.so"}
		if kind == "archives" {
			entries, err := os.ReadDir(source)
			if err != nil {
				return err
			}
			if len(entries) != 2 {
				return fmt.Errorf("installed archive cache is incomplete")
			}
			names = []string{entries[0].Name(), entries[1].Name()}
		}
		for _, name := range names {
			raw, err := safeio.Read(filepath.Join(source, name), 100_000_000)
			if err != nil {
				return err
			}
			if err := safeio.Atomic(filepath.Join(destination, name), raw, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}
