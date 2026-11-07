package resolver

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"vpn-script/internal/control/client/network"
)

// ConfigureNewLink changes only the caller's new TUN link. It never edits
// resolv.conf, global DNS policy, or another interface's existing configuration.
func (m *Manager) ConfigureNewLink(ctx context.Context, link Link) error {
	if err := m.validate(link); err != nil {
		return err
	}
	index := strconv.Itoa(link.Index)
	for _, args := range [][]string{{"dns", index, network.DNS}, {"domain", index, "~."},
		{"default-route", index, "yes"}, {"llmnr", index, "no"}, {"mdns", index, "no"}, {"dnsovertls", index, "no"}} {
		if err := m.command(ctx, args...); err != nil {
			rollback, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			return errors.Join(err, m.Revert(rollback, link))
		}
	}
	return nil
}

func (m *Manager) Revert(ctx context.Context, link Link) error {
	if err := m.validate(link); err != nil {
		return err
	}
	return m.command(ctx, "revert", strconv.Itoa(link.Index))
}

func (m *Manager) validate(link Link) error {
	if link.Index <= 0 || !regexp.MustCompile(`^nhop[a-z0-9_-]{0,10}$`).MatchString(link.Name) {
		return fmt.Errorf("invalid owned DNS link")
	}
	current, err := m.lookup(link.Name)
	if err != nil || current == nil || current.Name != link.Name || current.Index != link.Index {
		return fmt.Errorf("DNS link identity changed; refusing to configure or revert another interface")
	}
	return nil
}

func (m *Manager) command(ctx context.Context, args ...string) error {
	result, err := m.run(ctx, 5*time.Second, append([]string{"resolvectl"}, args...)...)
	if err != nil || !result.Success {
		return fmt.Errorf("resolved %s failed: %w", args[0], errors.Join(err, errors.New(string(result.Output))))
	}
	return nil
}
