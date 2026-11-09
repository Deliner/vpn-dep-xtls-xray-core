package runtimerelease

import (
	"bytes"
	"fmt"
	"strings"
)

type Inputs struct {
	Candidate string
	Native    string
	Arch      string
}

func verify(policy Policy, input Inputs) (map[string]string, error) {
	target, ok := policy.Targets[input.Arch]
	if !ok {
		return nil, fmt.Errorf("target must be reviewed Linux amd64 or arm64")
	}
	if err := verifyNative(policy, input, target); err != nil {
		return nil, err
	}
	entries, err := inventory(input.Candidate, "SHA256SUMS", target.InventorySHA256)
	if err != nil {
		return nil, err
	}
	for _, tree := range []string{"source", "bin"} {
		if err := completeTree(input.Candidate, tree, entries); err != nil {
			return nil, err
		}
	}
	if entries["bin/xray"] != target.BinarySHA256 {
		return nil, fmt.Errorf("candidate executable differs from native-tested bytes")
	}
	actual, err := readRegular(input.Candidate, "audit/actual.sha256", 4_000_000)
	if err != nil {
		return nil, err
	}
	if hash(actual) != target.InventorySHA256 {
		return nil, fmt.Errorf("audit did not cover the reviewed candidate inventory")
	}
	if err := verifyCandidate(policy, input); err != nil {
		return nil, err
	}
	return entries, nil
}

func verifyNative(policy Policy, input Inputs, target Target) error {
	entries, err := inventory(input.Native, "evidence.sha256", target.EvidenceSHA256)
	if err != nil {
		return err
	}
	if entries["xray-SHA256SUMS"] != target.InventorySHA256 {
		return fmt.Errorf("native inventory does not bind this candidate")
	}
	host, err := readRegular(input.Native, "host.txt", 1_000_000)
	if err != nil {
		return err
	}
	for _, line := range []string{"target=linux/" + input.Arch, "workflow_sha=" + policy.WorkflowSHA} {
		if !hasLine(host, line) {
			return fmt.Errorf("native run identity differs: %s", line)
		}
	}
	return exact(input.Native, "status", "passed\n")
}

func verifyCandidate(policy Policy, input Inputs) error {
	if err := exact(input.Candidate, "status", "built\n"); err != nil {
		return err
	}
	provenance, err := readRegular(input.Candidate, "provenance.txt", 1_000_000)
	if err != nil {
		return err
	}
	for _, line := range []string{"engine=xray", "base_version=" + policy.Version, "build_label=" + policy.Label,
		"target=linux/" + input.Arch, "source_url=" + policy.SourceURL, "source_archive_sha256=" + policy.SourceSHA256} {
		if !hasLine(provenance, line) {
			return fmt.Errorf("candidate provenance differs: %s", line)
		}
	}
	if err := exact(input.Candidate, "audit/provenance.txt", string(provenance)); err != nil {
		return err
	}
	for _, name := range []string{"source", "binary"} {
		raw, err := readRegular(input.Candidate, "audit/"+name+".txt", 4_000_000)
		if err != nil {
			return err
		}
		if !bytes.HasSuffix(raw, []byte("\nscanner_exit_status=0\n")) || bytes.Count(raw, []byte("scanner_exit_status=")) != 1 {
			return fmt.Errorf("candidate %s audit did not complete successfully", name)
		}
	}
	return nil
}

func hasLine(raw []byte, line string) bool {
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	return strings.Count("\n"+string(raw), "\n"+key+"=") == 1 &&
		strings.Contains("\n"+string(raw), "\n"+line+"\n")
}

func exact(root, name, want string) error {
	raw, err := readRegular(root, name, 4_000_000)
	if err != nil {
		return err
	}
	if string(raw) != want {
		return fmt.Errorf("unexpected %s", name)
	}
	return nil
}
