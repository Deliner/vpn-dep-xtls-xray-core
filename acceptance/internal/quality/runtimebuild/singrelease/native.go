package singrelease

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
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
			if name == "tuic/evidence.sha256" {
				return derivedTUICInventory(root, entries)
			}
			return fmt.Errorf("unrecorded native evidence input: %s", name)
		}
		return nil
	})
}

// Earlier producers excluded every evidence.sha256 basename. Authenticate this
// derived inventory from the already reviewed parent entries, never from itself.
func derivedTUICInventory(root string, entries map[string]string) error {
	var names []string
	for name := range entries {
		if strings.HasPrefix(name, "tuic/") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("TUIC inventory has no authenticated parent entries")
	}
	sort.Strings(names)
	var expected bytes.Buffer
	for _, name := range names {
		fmt.Fprintf(&expected, "%s  ./%s\n", entries[name], strings.TrimPrefix(name, "tuic/"))
	}
	raw, err := runtimerelease.ReadRegular(root, "tuic/evidence.sha256", 4_000_000)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, expected.Bytes()) {
		return fmt.Errorf("TUIC inventory differs from authenticated parent entries")
	}
	return nil
}
