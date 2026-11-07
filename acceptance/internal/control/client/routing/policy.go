package routing

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/resolver"
)

func (m *Manager) Apply(ctx context.Context, link resolver.Link) error {
	current, err := net.InterfaceByName(link.Name)
	if err != nil || current.Index != link.Index {
		return fmt.Errorf("TUN identity changed before route installation")
	}
	if err := m.availablePolicy(ctx); err != nil {
		return err
	}
	for _, r := range m.rules {
		if r.destination == "" {
			if err := m.command(ctx, "ip", r.family, "route", "add", "table", strconv.Itoa(network.RouteIndex), "default", "dev", link.Name, "proto", ownerProtocol); err != nil {
				return err
			}
		}
		if err := m.command(ctx, r.args("add")...); err != nil {
			return err
		}
	}
	return nil
}

// Restore runs after the TUN has disappeared, which removes its kernel routes.
// Only our exact rule selectors/protocol are deleted; foreign state keeps the
// kill switch active and must be inspected instead of being flushed.
func (m *Manager) Restore(ctx context.Context) error {
	for _, family := range []string{"-4", "-6"} {
		for _, table := range []int{network.RouteIndex, network.AssistTable} {
			if err := m.emptyTable(ctx, family, table); err != nil {
				return err
			}
		}
		items, err := m.inventory(ctx, family, "rule")
		if err != nil {
			return err
		}
		for _, value := range items {
			if reserved(value.Priority) || fmt.Sprint(value.Table) == strconv.Itoa(network.AssistTable) {
				if err := m.remove(ctx, family, value); err != nil {
					return err
				}
			}
		}
	}
	return m.vacant(ctx)
}

func (m *Manager) remove(ctx context.Context, family string, value item) error {
	if fmt.Sprint(value.Table) == strconv.Itoa(network.AssistTable) {
		return m.removeAuxiliary(ctx, family, value)
	}
	for _, r := range m.rules {
		if r.family == family && r.matches(value) {
			return m.command(ctx, r.args("del")...)
		}
	}
	return fmt.Errorf("reserved routing rule changed; refusing to remove foreign state")
}

func (m *Manager) removeAuxiliary(ctx context.Context, family string, value item) error {
	if family != "-6" || value.unknown || value.Source != "all" || value.Dest != "" || value.DestLen != 0 || value.OIF != m.interfaceName || value.Protocol != nil || value.Priority <= 0 {
		return fmt.Errorf("auxiliary TUN rule changed; refusing to remove foreign state")
	}
	return m.command(ctx, "ip", family, "rule", "del", "pref", strconv.Itoa(value.Priority), "table", strconv.Itoa(network.AssistTable), "oif", m.interfaceName)
}

func (m *Manager) command(ctx context.Context, args ...string) error {
	result, err := m.run(ctx, 5*time.Second, args...)
	if err != nil || !result.Success {
		return fmt.Errorf("client routing command failed: %w", err)
	}
	return nil
}
