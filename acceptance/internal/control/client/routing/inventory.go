package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"vpn-script/internal/control/client/network"
)

type item struct {
	Priority int             `json:"priority"`
	Source   string          `json:"src"`
	Dest     string          `json:"dst"`
	DestLen  int             `json:"dstlen"`
	Table    any             `json:"table"`
	Protocol any             `json:"protocol"`
	OIF      string          `json:"oif"`
	Detached json.RawMessage `json:"oif_detached"`
	unknown  bool
}

func (value *item) UnmarshalJSON(data []byte) error {
	type fields item
	if err := json.Unmarshal(data, (*fields)(value)); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	for key := range keys {
		switch key {
		case "priority", "src", "dst", "dstlen", "table", "protocol", "oif", "oif_detached":
		default:
			value.unknown = true
		}
	}
	return nil
}

func (m *Manager) inventory(ctx context.Context, family, object string) ([]item, error) {
	args := []string{"ip", family, "-j", object, "show"}
	if object == "route" {
		args = append(args, "table", "all")
	}
	result, err := m.run(ctx, 5*time.Second, args...)
	var items []item
	if err != nil || !result.Success || json.Unmarshal(result.Output, &items) != nil {
		return nil, fmt.Errorf("cannot inspect %s %s ownership", family, object)
	}
	return items, nil
}

func (m *Manager) emptyTable(ctx context.Context, family string, table int) error {
	items, err := m.inventory(ctx, family, "route")
	if err != nil {
		return err
	}
	for _, value := range items {
		if fmt.Sprint(value.Table) == strconv.Itoa(table) {
			return fmt.Errorf("reserved client route table is not empty; refusing to alter existing routes")
		}
	}
	return nil
}

func (m *Manager) vacant(ctx context.Context) error {
	if err := m.availablePolicy(ctx); err != nil {
		return err
	}
	for _, family := range []string{"-4", "-6"} {
		if err := m.emptyTable(ctx, family, network.AssistTable); err != nil {
			return err
		}
		items, err := m.inventory(ctx, family, "rule")
		if err != nil {
			return err
		}
		for _, value := range items {
			if fmt.Sprint(value.Table) == strconv.Itoa(network.AssistTable) {
				return fmt.Errorf("client auxiliary routing table is already referenced")
			}
		}
	}
	return nil
}

func (m *Manager) availablePolicy(ctx context.Context) error {
	for _, family := range []string{"-4", "-6"} {
		if err := m.emptyTable(ctx, family, network.RouteIndex); err != nil {
			return err
		}
		items, err := m.inventory(ctx, family, "rule")
		if err != nil {
			return err
		}
		for _, value := range items {
			if reserved(value.Priority) {
				return fmt.Errorf("client routing priority already exists; refusing to adopt it")
			}
		}
	}
	return nil
}

func reserved(priority int) bool {
	return priority >= network.RouteIndex && priority <= network.RouteIndex+10
}

func (r rule) matches(value item) bool {
	if value.unknown || value.OIF != "" || value.Detached != nil || value.Priority != r.priority || value.Source != "all" || fmt.Sprint(value.Table) != r.table || fmt.Sprint(value.Protocol) != ownerProtocol {
		return false
	}
	if r.destination == "" {
		return value.Dest == "" || value.Dest == "all"
	}
	expected := netip.MustParsePrefix(r.destination)
	return value.Dest == expected.Addr().String() && (value.DestLen == 0 || value.DestLen == expected.Bits())
}
