package singrelease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

type Target struct {
	Binary      string `json:"binary_sha256"`
	Library     string `json:"library_sha256"`
	Inventory   string `json:"inventory_sha256"`
	Evidence    string `json:"evidence_sha256"`
	Provenance  string `json:"provenance_sha256"`
	SourceAudit string `json:"source_audit_sha256"`
	BinaryAudit string `json:"binary_audit_sha256"`
}

type Inputs struct {
	Arch      string
	Candidate string
	Native    string
}

type Policy struct {
	Version   string            `json:"version"`
	Label     string            `json:"label"`
	SourceURL string            `json:"source_url"`
	SourceSHA string            `json:"source_sha256"`
	Workflow  string            `json:"workflow_sha"`
	Run       string            `json:"run_id"`
	Targets   map[string]Target `json:"targets"`
}

const tuicLabel = "nhop-fork-cronet154-zlib-tuic-1"

func Load(path string) (Policy, error) {
	var policy Policy
	raw, err := safeio.Read(path, 32768)
	if err != nil {
		return policy, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return policy, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return policy, fmt.Errorf("expected one sing-box release policy")
	}
	return policy, policy.valid()
}

func (p Policy) valid() error {
	const source = "https://codeload.github.com/Deliner/vpn-dep-sagernet-sing-box/tar.gz/"
	commit, ok := strings.CutPrefix(p.SourceURL, source)
	if !ok || !commitSHA(commit) || !commitSHA(p.Workflow) || !validate.Hash(p.SourceSHA) {
		return fmt.Errorf("release requires exact owned sing-box source and workflow")
	}
	if p.Version != "1.14.4" || (p.Label != "nhop-fork-cronet154-zlib-1" && p.Label != tuicLabel) || len(p.Targets) != 2 {
		return fmt.Errorf("unreviewed sing-box release generation")
	}
	if p.Run == "" || p.Run[0] == '0' || strings.Trim(p.Run, "0123456789") != "" {
		return fmt.Errorf("invalid native run ID")
	}
	for _, arch := range []string{"amd64", "arm64"} {
		target := p.Targets[arch]
		for _, sha := range []string{target.Binary, target.Library, target.Inventory, target.Evidence, target.Provenance, target.SourceAudit, target.BinaryAudit} {
			if !validate.Hash(sha) {
				return fmt.Errorf("incomplete sing-box target: %s", arch)
			}
		}
	}
	return nil
}

func commitSHA(value string) bool {
	return len(value) == 40 && validate.Hash(value+strings.Repeat("0", 24))
}
