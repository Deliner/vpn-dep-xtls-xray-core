package singrelease

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"vpn-script/internal/quality/runtimerelease"
)

func verifyNative(policy Policy, input Inputs, target Target) error {
	entries, err := runtimerelease.VerifyInventory(input.Native, "evidence.sha256", target.Evidence)
	if err != nil {
		return err
	}
	if entries["sing-box-SHA256SUMS"] != target.Inventory || entries["sing-box-provenance.txt"] != target.Provenance {
		return fmt.Errorf("native run did not use this candidate")
	}
	if err := completeEvidence(input.Native, entries); err != nil {
		return err
	}
	if err := exact(input.Native, "status", "passed\n"); err != nil {
		return err
	}
	host, err := runtimerelease.ReadRegular(input.Native, "host.txt", 1_000_000)
	if err != nil {
		return err
	}
	return requireLines(host, []string{"target=linux/" + input.Arch, "workflow_sha=" + policy.Workflow, "run_id=" + policy.Run})
}

func completeEvidence(root string, entries map[string]string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)
		if !entry.Type().IsRegular() {
			return fmt.Errorf("special native evidence input: %s", name)
		}
		if entries[name] == "" && name != "evidence.sha256" && name != "integrity-verification.log" {
			return fmt.Errorf("unrecorded native evidence input: %s", name)
		}
		return nil
	})
}
