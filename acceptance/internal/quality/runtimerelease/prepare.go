package runtimerelease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"vpn-script/internal/platform/safeio"
)

type Receipt struct {
	Schema       int    `json:"schema_version"`
	Engine       string `json:"engine"`
	Version      string `json:"version"`
	Label        string `json:"build_label"`
	Platform     string `json:"platform"`
	SourceURL    string `json:"source_url"`
	SourceSHA256 string `json:"source_sha256"`
	NativeRun    string `json:"native_run"`
	WorkflowSHA  string `json:"workflow_sha"`
	Target       Target `json:"tested_inputs"`
	Archive      string `json:"archive"`
	SHA256       string `json:"sha256,omitempty"`
	Size         int    `json:"size,omitempty"`
}

// Prepare creates a new private directory only after binding every input to native CI.
// A failed attempt removes only the directory it created; an existing output is never replaced.
func Prepare(policy Policy, input Inputs, output string) (err error) {
	if err := policy.validate(); err != nil {
		return err
	}
	entries, err := verify(policy, input)
	if err != nil {
		return err
	}
	if entries["source/LICENSE"] == "" || entries["source/go.mod"] == "" || entries["source/go.sum"] == "" {
		return fmt.Errorf("source license and module locks are required")
	}
	if err := os.Mkdir(output, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(output)
		}
	}()
	return prepareArchive(policy, input, output, entries)
}

func prepareArchive(policy Policy, input Inputs, output string, entries map[string]string) error {
	receipt := identity(policy, input.Arch)
	metadata, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	if err := archive(filepath.Join(output, receipt.Archive), input.Candidate, entries, metadata); err != nil {
		return err
	}
	raw, err := readRegular(output, receipt.Archive, 150_000_000)
	if err != nil {
		return err
	}
	receipt.SHA256, receipt.Size = hash(raw), len(raw)
	return saveReceipt(output, receipt)
}

func identity(policy Policy, arch string) Receipt {
	return Receipt{Schema: 1, Engine: "xray", Version: policy.Version, Label: policy.Label,
		Platform: "linux/" + arch, SourceURL: policy.SourceURL, SourceSHA256: policy.SourceSHA256,
		NativeRun: policy.RunURL, WorkflowSHA: policy.WorkflowSHA, Target: policy.Targets[arch],
		Archive: "Xray-linux-" + arch + ".zip"}
}

func saveReceipt(output string, receipt Receipt) error {
	raw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	if err := safeio.Create(filepath.Join(output, "receipt.json"), append(raw, '\n'), 0600); err != nil {
		return err
	}
	sums := fmt.Sprintf("%s  %s\n%s  receipt.json\n", receipt.SHA256, receipt.Archive, hash(append(raw, '\n')))
	return safeio.Create(filepath.Join(output, "SHA256SUMS"), []byte(sums), 0600)
}
