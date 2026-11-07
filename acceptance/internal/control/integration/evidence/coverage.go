package evidence

import (
	"fmt"
	"strings"
)

func (r report) checkNodeAddresses(expected []string) error {
	if len(expected) == 0 {
		expected = []string{"127.0.0.1"}
	}
	if len(r.NodeAddresses) != r.Hops || (len(expected) != 1 && len(expected) != r.Hops) {
		return fmt.Errorf("incomplete or ambiguous node addresses")
	}
	for hop, actual := range r.NodeAddresses {
		address := expected[hop%len(expected)]
		if (address != "127.0.0.1" && address != "::1") || actual != address {
			return fmt.Errorf("unexpected node%d address: %q, expected %q", hop, actual, address)
		}
	}
	return nil
}

func (r report) checkCoverage(expected []string) error {
	known := map[string]bool{}
	for _, name := range expected {
		if name == "" || known[name] {
			return fmt.Errorf("ambiguous expected profiles")
		}
		known[name] = true
	}
	if r.Services != r.Hops*len(expected) || len(r.Profiles) != len(expected) || len(r.Paths) != len(expected) || !known[r.Stopped] {
		return fmt.Errorf("incomplete profile/service coverage")
	}
	seen, services := map[string]bool{}, map[string]bool{}
	for _, name := range r.Profiles {
		if !known[name] || seen[name] || len(r.Paths[name]) != r.Hops {
			return fmt.Errorf("unknown, duplicate or incomplete profile %s", name)
		}
		seen[name] = true
		for hop, service := range r.Paths[name] {
			node, profile, found := strings.Cut(service, ".")
			if !found || node != fmt.Sprintf("node%d", hop) || !known[profile] || services[service] {
				return fmt.Errorf("invalid route %s at hop %d", name, hop)
			}
			services[service] = true
		}
	}
	return nil
}

func (r report) checkPhases(expected []string) error {
	if len(r.Phases) != 3 {
		return fmt.Errorf("incomplete concurrent phases")
	}
	for i, name := range []string{"all-running", "one-client-stopped", "all-restored"} {
		phase := r.Phases[i]
		active := map[string]bool{}
		for _, profile := range expected {
			if i != 1 || profile != r.Stopped {
				active[profile] = true
			}
		}
		if phase.Name != name || phase.Profiles != len(active) || phase.Streams != 2*len(active) || phase.Required != 25 || phase.Barriers != 25 {
			return fmt.Errorf("incomplete concurrency/barriers for phase %s", name)
		}
		if phase.TCPBytes != 32768 || phase.UDPBytes != 512 || len(phase.Missing) != 0 {
			return fmt.Errorf("wrong payload size or missing arrivals for phase %s", name)
		}
		if err := phase.checkRoutes(active); err != nil {
			return fmt.Errorf("phase %s: %w", name, err)
		}
	}
	return nil
}

func (p phase) checkRoutes(expected map[string]bool) error {
	if len(p.Routes) != len(expected) {
		return fmt.Errorf("incomplete active routes")
	}
	for name, result := range p.Routes {
		if !expected[name] || result.TCPRounds != 25 || result.UDPRounds != 25 || result.Error != "" {
			return fmt.Errorf("incomplete or failed traffic for %s", name)
		}
	}
	return nil
}
