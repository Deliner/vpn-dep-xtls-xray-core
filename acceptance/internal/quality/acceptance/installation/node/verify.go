package node

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func Verify(directory, arch, commit, snapshot string) error {
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unsupported installer acceptance architecture")
	}
	if err := completeInventory(directory); err != nil {
		return err
	}
	if err := identity(directory, snapshot, arch, commit); err != nil {
		return err
	}
	if err := textFile(filepath.Join(directory, "status"), "passed"); err != nil {
		return err
	}
	if err := hostCleanup(directory); err != nil {
		return err
	}
	previous := filepath.Join(directory, "previous")
	if err := previousSources(previous, snapshot, arch); err != nil {
		return err
	}
	return verifyInstallation(directory, arch, snapshot)
}

func verifyInstallation(directory, arch, snapshot string) error {
	packagePath := filepath.Join(directory, "package")
	bundle := filepath.Join(packagePath, "payload")
	reports := filepath.Join(bundle, "reports")
	result, err := verifyReport(filepath.Join(reports, "installed.json"))
	if err != nil {
		return err
	}
	if err := verifyPackage(packagePath, snapshot, arch, result); err != nil {
		return err
	}
	if err := equalFiles(filepath.Join(directory, "previous/bin/nhop-node"), filepath.Join(bundle, "bin/previous-nhop-node")); err != nil {
		return err
	}
	if err := verifyIntegrityLog(bundle); err != nil {
		return err
	}
	if err := verifyCrash(reports, result); err != nil {
		return err
	}
	if err := verifyRuntimes(reports, arch, result); err != nil {
		return err
	}
	return verifyExecution(directory, reports, len(result.Profiles))
}

func identity(directory, snapshot, arch, commit string) error {
	observed, err := properties(filepath.Join(directory, "identity.txt"))
	if err != nil {
		return err
	}
	source, err := properties(filepath.Join(snapshot, "source.txt"))
	if err != nil {
		return err
	}
	machine := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[arch]
	if len(observed) != 5 || observed["workflow_sha"] != commit || observed["project_commit"] != source["project_commit"] ||
		observed["target"] != "linux/"+arch || observed["machine"] != machine {
		return fmt.Errorf("native installer identity does not match the reviewed CI snapshot")
	}
	run, err := strconv.ParseUint(observed["run_id"], 10, 64)
	if err != nil || run == 0 {
		return fmt.Errorf("invalid native installer run ID")
	}
	return equalFiles(filepath.Join(directory, "source.txt"), filepath.Join(snapshot, "source.txt"))
}

func hostCleanup(directory string) error {
	for _, name := range []string{"clean-before.txt", "clean-after.txt"} {
		if err := textFile(filepath.Join(directory, name), "service/account/group/paths/processes absent"); err != nil {
			return err
		}
	}
	before := filepath.Join(directory, "host-before.txt")
	raw, err := safeio.Read(before, 1<<20)
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("host baseline missing")
	}
	if err := baseline(raw); err != nil {
		return err
	}
	return equalFiles(before, filepath.Join(directory, "host-after.txt"))
}

func baseline(raw []byte) error {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	paths := []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow", "/etc/resolv.conf"}
	if len(lines) != len(paths)+4 {
		return fmt.Errorf("incomplete host baseline")
	}
	for index, path := range paths {
		sha, name, ok := strings.Cut(lines[index], "  ")
		if !ok || name != path || !validate.Hash(sha) {
			return fmt.Errorf("invalid host file identity: %s", path)
		}
	}
	for _, line := range lines[len(paths):] {
		var entries []map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &entries); err != nil || entries == nil {
			return fmt.Errorf("missing route or rule baseline")
		}
	}
	return nil
}
