package journal

import (
	"fmt"
	"path/filepath"
	"vpn-script/internal/core/validate"
)

const Name = "transaction.json"

type Snapshot struct {
	Previous string `json:"release"`
	Unit     []byte `json:"unit"`
	Firewall []byte `json:"firewall"`
	State    []byte `json:"state"`
	Active   bool   `json:"active"`
	Enabled  bool   `json:"enabled"`
}

type Record struct {
	Schema          int      `json:"schema_version"`
	Phase           string   `json:"phase"`
	TopologyID      string   `json:"topology_id"`
	NodeID          string   `json:"node_id"`
	FirewallChanged bool     `json:"firewall_changed"`
	Old             Snapshot `json:"old"`
	Next            Snapshot `json:"next"`
}

func (r Record) validate(base string) error {
	if r.Schema != 1 || r.Phase != "pending" && r.Phase != "committed" ||
		validate.Name(r.TopologyID) != nil || validate.Name(r.NodeID) != nil {
		return fmt.Errorf("invalid installer transaction identity or phase")
	}
	for _, target := range []string{r.Old.Previous, r.Next.Previous} {
		if target != "" && (!validate.Hash(filepath.Base(target)) || target != filepath.Join(base, "releases", filepath.Base(target))) {
			return fmt.Errorf("transaction release escapes managed history")
		}
	}
	if r.Next.Previous == "" || len(r.Next.State) == 0 || len(r.Next.Unit) == 0 {
		return fmt.Errorf("transaction requires complete intended state")
	}
	return nil
}
