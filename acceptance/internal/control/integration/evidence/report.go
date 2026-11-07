package evidence

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"vpn-script/internal/platform/safeio"
)

// CheckReports rejects skipped, incomplete and stale evidence even if go test exited zero.
// It checks the saved contract; it does not attest an untrusted producer or remote host.
// An empty platform requires the current OS/CPU; an explicit one is never inferred from JSON.
func CheckReports(directory string, hops []int, profiles []string, since time.Time, platform string, addresses ...string) error {
	if platform == "" {
		platform = runtime.GOOS + "/" + runtime.GOARCH
	}
	switch platform {
	case "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64":
	default:
		return fmt.Errorf("unsupported expected payload platform %q", platform)
	}
	if len(hops) == 0 || len(profiles) == 0 {
		return fmt.Errorf("empty expected payload coverage")
	}
	seen := map[int]bool{}
	for _, count := range hops {
		if (count != 1 && count != 3) || seen[count] {
			return fmt.Errorf("unsupported or duplicate hop count %d", count)
		}
		seen[count] = true
		path := filepath.Join(directory, fmt.Sprintf("%d-hop.json", count))
		data, err := safeio.Read(path, 4*1024*1024)
		if err != nil {
			return fmt.Errorf("payload report %s: %w", path, err)
		}
		var value report
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode payload report %s: %w", path, err)
		}
		if err := value.check(count, profiles, since, platform, addresses...); err != nil {
			return fmt.Errorf("payload report %s: %w", path, err)
		}
	}
	return nil
}

func (r report) check(hops int, profiles []string, since time.Time, platform string, addresses ...string) error {
	if r.SchemaVersion != 4 || r.Status != "passed" || r.Platform != platform || r.TestGoVersion != runtime.Version() || r.Hops != hops {
		return fmt.Errorf("unexpected schema, status, platform or hop count")
	}
	if r.StartedAt.IsZero() || r.StartedAt.Before(since) || r.FinishedAt.Before(r.StartedAt) || r.FinishedAt.After(time.Now().Add(time.Minute)) {
		return fmt.Errorf("stale or invalid report timestamps")
	}
	if err := r.checkNodeAddresses(addresses); err != nil {
		return err
	}
	if err := r.checkCoverage(profiles); err != nil {
		return err
	}
	if err := r.checkAssignments(); err != nil {
		return err
	}
	return r.checkPhases(profiles)
}

func (r report) checkAssignments() error {
	if len(r.Assignments.Nodes) != r.Hops {
		return fmt.Errorf("incomplete node runtime assignments")
	}
	used := map[string]bool{}
	if err := r.checkParticipant("clients", r.Assignments.Clients, used); err != nil {
		return err
	}
	for node := range r.Hops {
		name := fmt.Sprintf("node%d", node)
		if err := r.checkParticipant(name, r.Assignments.Nodes[name], used); err != nil {
			return err
		}
	}
	if len(used) != len(r.Runtimes) {
		return fmt.Errorf("unassigned runtime identities in report")
	}
	return nil
}

func (r report) checkParticipant(name string, bindings map[string]string, used map[string]bool) error {
	if len(bindings) != 2 {
		return fmt.Errorf("missing engine assignments for %s", name)
	}
	for _, engine := range []string{"xray", "sing-box"} {
		id := bindings[engine]
		role, kind, found := strings.Cut(id, "/")
		validRole := name == "clients" && role == "client" || name != "clients" && (role == "server" || role == "updated")
		if !found || kind != engine || !validRole {
			return fmt.Errorf("wrong runtime role or engine for %s/%s", name, engine)
		}
		observed, exists := r.Runtimes[id]
		digest, err := hex.DecodeString(observed.SHA256)
		if !exists || id == "" || err != nil || len(digest) != 32 || !filepath.IsAbs(observed.Path) {
			return fmt.Errorf("invalid runtime identity for %s/%s", name, engine)
		}
		if !observed.VerifiedUnchanged || observed.Error != "" || observed.VersionOutput == "" || observed.ExpectedVersion == "" {
			return fmt.Errorf("unverified runtime identity %s", id)
		}
		used[id] = true
	}
	return nil
}
