package runtimeaudit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"vpn-script/internal/platform/process"
)

type queryEvidence struct {
	configured bool
	expected   map[string]bool
	observed   map[string]bool
}

func verifyReplacementQuery(raw []byte, queries []string) error {
	if len(raw) >= process.OutputLimit || len(queries) == 0 {
		return fmt.Errorf("upstream query output may be truncated or has no requested modules")
	}
	state := queryEvidence{expected: map[string]bool{}, observed: map[string]bool{}}
	for _, query := range queries {
		module, version, ok := strings.Cut(query, "@")
		if !ok || module == "" || version == "" {
			return fmt.Errorf("invalid upstream query identity")
		}
		state.expected["Looking up vulnerabilities in "+module+" at "+version+"..."] = true
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	for {
		var message map[string]json.RawMessage
		err := decoder.Decode(&message)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if err := state.message(message); err != nil {
			return err
		}
	}
	if !state.configured || len(state.observed) != len(state.expected) {
		return fmt.Errorf("incomplete upstream module query evidence")
	}
	return nil
}

func (state *queryEvidence) message(message map[string]json.RawMessage) error {
	if len(message) != 1 {
		return fmt.Errorf("unexpected upstream query message")
	}
	for kind, raw := range message {
		switch kind {
		case "config":
			return state.configuration(raw)
		case "progress":
			return state.progress(raw)
		case "osv":
			var advisory struct{ ID string }
			if err := json.Unmarshal(raw, &advisory); err != nil || advisory.ID == "" {
				return fmt.Errorf("invalid upstream advisory: %v", err)
			}
			return fmt.Errorf("upstream module advisory requires applicability review: %s", advisory.ID)
		default:
			return fmt.Errorf("unsupported upstream query message: %s", kind)
		}
	}
	return nil
}

func (state *queryEvidence) configuration(raw []byte) error {
	var config struct {
		Protocol string `json:"protocol_version"`
		Scanner  string `json:"scanner_name"`
		Mode     string `json:"scan_mode"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	if state.configured || config.Protocol != "v1.0.0" || config.Scanner != "govulncheck" || config.Mode != "query" {
		return fmt.Errorf("unexpected upstream query configuration")
	}
	state.configured = true
	return nil
}

func (state *queryEvidence) progress(raw []byte) error {
	var progress struct{ Message string }
	if err := json.Unmarshal(raw, &progress); err != nil {
		return err
	}
	if !state.configured || !state.expected[progress.Message] || state.observed[progress.Message] {
		return fmt.Errorf("missing, duplicate or unexpected upstream module lookup")
	}
	state.observed[progress.Message] = true
	return nil
}
