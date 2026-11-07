package guard

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"vpn-script/internal/control/client/network"
)

type listing struct {
	NFTables []struct {
		Table *struct {
			Family  string `json:"family"`
			Name    string `json:"name"`
			Handle  uint64 `json:"handle"`
			Comment string `json:"comment"`
		} `json:"table"`
		Chain *struct {
			Family string `json:"family"`
			Table  string `json:"table"`
			Name   string `json:"name"`
			Type   string `json:"type"`
			Hook   string `json:"hook"`
			Prio   int    `json:"prio"`
			Policy string `json:"policy"`
		} `json:"chain"`
	} `json:"nftables"`
}

func (m *Manager) verifyProtection(ctx context.Context) error {
	result, err := m.run(ctx, 5*time.Second, "nft", "--json", "list", "table", "inet", network.Table)
	var value listing
	if err != nil || !result.Success || json.Unmarshal(result.Output, &value) != nil {
		return fmt.Errorf("could not verify installed firewall policy; client must not start")
	}
	seen := map[string]bool{}
	for _, item := range value.NFTables {
		c := item.Chain
		if c != nil && c.Family == "inet" && c.Table == network.Table && c.Type == "filter" && c.Prio == 50 && c.Policy == "drop" && c.Hook == c.Name {
			seen[c.Name] = true
		}
	}
	if !seen["output"] || !seen["forward"] {
		return fmt.Errorf("required output/forward drop chains missing; client must not start")
	}
	return nil
}

func (m *Manager) inspect(ctx context.Context) (Lease, error) {
	result, err := m.run(ctx, 5*time.Second, "nft", "--json", "--handle", "list", "table", "inet", network.Table)
	if err != nil || !result.Success {
		return Lease{}, fmt.Errorf("cannot inspect client kill switch")
	}
	var value listing
	if err := json.Unmarshal(result.Output, &value); err != nil {
		return Lease{}, fmt.Errorf("invalid nftables identity response")
	}
	var matches []Lease
	for _, item := range value.NFTables {
		if table := item.Table; table != nil && table.Name == network.Table && table.Family == "inet" {
			matches = append(matches, Lease{Token: table.Comment, Handle: table.Handle})
		}
	}
	if len(matches) != 1 || matches[0].Handle == 0 {
		return Lease{}, fmt.Errorf("ambiguous client kill switch identity")
	}
	return matches[0], nil
}

// Release removes exactly the table originally acquired. The kernel handle
// prevents a same-name replacement between inspection and deletion from being
// deleted. Do not call this on an unexpected client/engine shutdown.
func (m *Manager) Release(ctx context.Context, lease Lease) error {
	if !regexp.MustCompile(`^nhop:[0-9a-f]{32}$`).MatchString(lease.Token) || lease.Handle == 0 {
		return fmt.Errorf("invalid kill switch lease")
	}
	current, err := m.inspect(ctx)
	if err != nil || current != lease {
		return fmt.Errorf("client kill switch identity changed; protection retained")
	}
	result, err := m.run(ctx, 5*time.Second, "nft", "delete", "table", "inet", "handle", strconv.FormatUint(lease.Handle, 10))
	if err != nil || !result.Success {
		return fmt.Errorf("cannot remove owned kill switch; protection retained: %w", err)
	}
	return nil
}
