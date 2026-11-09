package singrelease

import (
	"bytes"
	"fmt"
	"strings"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/runtimerelease"
)

// Verify binds complete candidate inputs, successful audits and the native
// execution to explicit reviewed hashes. It neither publishes nor runs engines.
func Verify(policy Policy, input Inputs) (map[string]string, error) {
	if err := policy.valid(); err != nil {
		return nil, err
	}
	target, ok := policy.Targets[input.Arch]
	if !ok {
		return nil, fmt.Errorf("unreviewed CPU")
	}
	entries, err := runtimerelease.VerifyInventory(input.Candidate, "SHA256SUMS", target.Inventory, "source", "bin")
	if err != nil {
		return nil, err
	}
	if entries["bin/sing-box"] != target.Binary || entries["bin/libcronet.so"] != target.Library {
		return nil, fmt.Errorf("native-tested binary or Cronet differs")
	}
	for _, name := range []string{"source/LICENSE", "source/go.mod", "source/go.sum"} {
		if entries[name] == "" {
			return nil, fmt.Errorf("missing required source input: %s", name)
		}
	}
	if err := verifyCandidate(policy, input, target); err != nil {
		return nil, err
	}
	if err := verifyNative(policy, input, target); err != nil {
		return nil, err
	}
	return entries, nil
}

// VerifyMetadata authenticates retained evidence before restoring files omitted
// by an artifact transporter. It is not a complete candidate acceptance gate.
func VerifyMetadata(policy Policy, input Inputs) error {
	if err := policy.valid(); err != nil {
		return err
	}
	target, ok := policy.Targets[input.Arch]
	if !ok {
		return fmt.Errorf("unreviewed CPU")
	}
	raw, err := runtimerelease.ReadRegular(input.Candidate, "SHA256SUMS", 4_000_000)
	if err != nil || safeio.Hash(raw) != target.Inventory {
		return fmt.Errorf("retained candidate inventory differs")
	}
	if err := verifyCandidate(policy, input, target); err != nil {
		return err
	}
	return verifyNative(policy, input, target)
}

func verifyCandidate(policy Policy, input Inputs, target Target) error {
	if err := exact(input.Candidate, "status", "built\n"); err != nil {
		return err
	}
	for path, sha := range map[string]string{"provenance.txt": target.Provenance, "audit/provenance.txt": target.Provenance,
		"audit/actual.sha256": target.Inventory, "audit/source.txt": target.SourceAudit, "audit/binary.txt": target.BinaryAudit} {
		raw, err := runtimerelease.ReadRegular(input.Candidate, path, 4_000_000)
		if err != nil || safeio.Hash(raw) != sha {
			return fmt.Errorf("candidate metadata differs: %s", path)
		}
		if strings.HasSuffix(path, ".txt") && (path == "audit/source.txt" || path == "audit/binary.txt") {
			if !bytes.HasSuffix(raw, []byte("\nscanner_exit_status=0\n")) || bytes.Count(raw, []byte("scanner_exit_status=")) != 1 {
				return fmt.Errorf("unsuccessful candidate audit: %s", path)
			}
		}
	}
	provenance, err := runtimerelease.ReadRegular(input.Candidate, "provenance.txt", 1_000_000)
	if err != nil {
		return err
	}
	lines := []string{"engine=sing-box", "base_version=" + policy.Version, "build_label=" + policy.Label,
		"target=linux/" + input.Arch, "source_url=" + policy.SourceURL, "source_archive_sha256=" + policy.SourceSHA,
		"loaded_companion_sha256=" + target.Library, "build_cgo=0"}
	return requireLines(provenance, lines)
}
