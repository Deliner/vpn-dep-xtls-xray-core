package runtimerelease

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"vpn-script/internal/platform/safeio"
)

type Target struct {
	BinarySHA256    string `json:"binary_sha256"`
	InventorySHA256 string `json:"inventory_sha256"`
	EvidenceSHA256  string `json:"evidence_sha256"`
}

type Policy struct {
	Version      string            `json:"version"`
	Label        string            `json:"label"`
	SourceURL    string            `json:"source_url"`
	SourceSHA256 string            `json:"source_sha256"`
	RunURL       string            `json:"run_url"`
	WorkflowSHA  string            `json:"workflow_sha"`
	Targets      map[string]Target `json:"targets"`
}

func LoadPolicy(path string) (Policy, error) {
	var policy Policy
	raw, err := safeio.Read(path, 32_768)
	if err != nil {
		return policy, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return policy, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return policy, fmt.Errorf("policy must contain one JSON object")
	}
	return policy, policy.validate()
}

func (p Policy) validate() error {
	if p.Version == "" || strings.ContainsAny(p.Version, "/\\\x00\r\n") || !strings.HasPrefix(p.Label, "nhop-fork-") {
		return fmt.Errorf("policy requires an explicit maintained runtime identity")
	}
	if !strings.HasPrefix(p.SourceURL, "https://codeload.github.com/Deliner/vpn-dep-xtls-xray-core/tar.gz/") ||
		!strings.HasPrefix(p.RunURL, "https://github.com/Deliner/vpn-dep-xtls-xray-core/actions/runs/") {
		return fmt.Errorf("policy requires owned Xray source and native run")
	}
	if !digest(p.SourceSHA256, 32) || !digest(p.WorkflowSHA, 20) || len(p.Targets) != 2 {
		return fmt.Errorf("policy requires exact source, workflow and two native targets")
	}
	for _, arch := range []string{"amd64", "arm64"} {
		target := p.Targets[arch]
		if !digest(target.BinarySHA256, 32) || !digest(target.InventorySHA256, 32) || !digest(target.EvidenceSHA256, 32) {
			return fmt.Errorf("incomplete reviewed native target: %s", arch)
		}
	}
	return nil
}

func digest(value string, size int) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == size && value == strings.ToLower(value)
}
