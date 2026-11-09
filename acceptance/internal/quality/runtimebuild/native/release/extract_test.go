package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractionNeverPublishesAnUnreviewedLibrary(t *testing.T) {
	for _, fault := range []string{"", "policy", "cpu", "archive", "existing"} {
		t.Run(fault, func(t *testing.T) {
			policy, root := stageFixture(t)
			arch, output := "amd64", filepath.Join(root, "libcronet.so")
			switch fault {
			case "policy":
				policy.Targets[arch] = Target{}
			case "cpu":
				arch = "other"
			case "archive":
				put(t, root, archiveName(arch), []byte("unreviewed"))
			case "existing":
				put(t, root, "libcronet.so", []byte("existing bytes"))
			}
			err := ExtractLibrary(policy, arch, filepath.Join(root, archiveName("amd64")), output)
			if (err == nil) != (fault == "") {
				t.Fatal(fault, err)
			}
			raw, readErr := os.ReadFile(output)
			switch fault {
			case "":
				info, err := os.Stat(output)
				if readErr != nil || string(raw) != "reviewed library" || err != nil || info.Mode().Perm() != 0755 {
					t.Fatal("reviewed library not delivered", readErr, err)
				}
			case "existing":
				if readErr != nil || string(raw) != "existing bytes" {
					t.Fatal("existing library changed", readErr)
				}
			default:
				if !os.IsNotExist(readErr) {
					t.Fatal("failed verification left a library", readErr)
				}
			}
		})
	}
}
