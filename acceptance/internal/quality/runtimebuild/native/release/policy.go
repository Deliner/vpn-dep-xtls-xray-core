package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

type Target struct {
	Naive   string `json:"naive_evidence_sha256"`
	Zlib    string `json:"zlib_evidence_sha256"`
	Build   string `json:"build_evidence_sha256"`
	Native  string `json:"native_evidence_sha256"`
	Library string `json:"library_sha256"`
}

type Policy struct {
	Workflow string            `json:"workflow_sha"`
	Run      string            `json:"run_id"`
	Sources  string            `json:"source_pins_sha256"`
	Targets  map[string]Target `json:"targets"`
}

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
		return policy, fmt.Errorf("expected one Cronet release policy")
	}
	return policy, policy.valid()
}

func (p Policy) valid() error {
	if len(p.Workflow) != 40 || !validate.Hash(p.Workflow+"000000000000000000000000") || !validate.Hash(p.Sources) || len(p.Targets) != 2 {
		return fmt.Errorf("incomplete reviewed Cronet identity")
	}
	if p.Run == "" || p.Run[0] == '0' {
		return fmt.Errorf("invalid native run ID")
	}
	for _, c := range p.Run {
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid native run ID")
		}
	}
	for _, arch := range []string{"amd64", "arm64"} {
		t := p.Targets[arch]
		if !validate.Hash(t.Build) || !validate.Hash(t.Native) || !validate.Hash(t.Library) || !validate.Hash(t.Naive) || !validate.Hash(t.Zlib) {
			return fmt.Errorf("incomplete Cronet target: %s", arch)
		}
	}
	return nil
}
