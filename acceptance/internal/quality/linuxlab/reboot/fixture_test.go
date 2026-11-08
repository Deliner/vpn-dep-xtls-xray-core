package reboot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

var roles = []string{"boot-client", "boot-exit"}

type fixture struct {
	root, repository, lima string
	config                 map[string]contract.Config
	boots                  map[string]string
}

func hostFixture(t *testing.T) *fixture {
	t.Helper()
	root, repository := os.Getenv("NHOP_REBOOT_ROOT"), os.Getenv("NHOP_REBOOT_REPOSITORY")
	if root == "" {
		t.Skip("use just client-reboot-test with the two owned Linux VMs")
	}
	if runtime.GOOS != "darwin" || !filepath.IsAbs(repository) || filepath.Dir(root) != filepath.Join(repository, "build/client-reboot-lab/runs") || !strings.HasPrefix(filepath.Base(root), "run-") {
		t.Fatal("real reboot acceptance requires the owned macOS Lima lab")
	}
	require(t, safeio.CheckAccess(root, os.ModeDir|0700, os.Geteuid(), -1))
	home, err := os.UserHomeDir()
	require(t, err)
	metadata := os.Getenv("LIMA_HOME")
	if filepath.Dir(metadata) != filepath.Join(home, ".ssh-custom") || !strings.HasPrefix(filepath.Base(metadata), "nhop-clientboot-") {
		t.Fatal("unexpected reboot SSH metadata")
	}
	owner, err := safeio.Read(filepath.Join(metadata, ".nhop-owner"), 4096)
	require(t, err)
	if strings.TrimSpace(string(owner)) != repository {
		t.Fatal("reboot lab belongs to another checkout")
	}
	return &fixture{root: root, repository: repository, lima: filepath.Join(repository, "build/lab/tools/bin/limactl"),
		config: map[string]contract.Config{}, boots: map[string]string{}}
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var result T
	require(t, json.Unmarshal(raw, &result))
	return result
}
