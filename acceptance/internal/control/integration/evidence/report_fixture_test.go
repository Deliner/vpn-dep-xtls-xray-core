package evidence

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/platform/safeio"
)

func successfulReport(hops int, since time.Time) report {
	r := report{SchemaVersion: 4, Status: "passed", Platform: runtime.GOOS + "/" + runtime.GOARCH,
		TestGoVersion: runtime.Version(), Hops: hops, Services: 2 * hops, Profiles: []string{"a", "b"},
		Stopped: "a", StartedAt: since, FinishedAt: since.Add(time.Millisecond),
		Paths: map[string][]string{}, Runtimes: map[string]Runtime{},
		Assignments: assignments{Clients: map[string]string{}, Nodes: map[string]map[string]string{}}}
	for range hops {
		r.NodeAddresses = append(r.NodeAddresses, "127.0.0.1")
	}
	for index, name := range r.Profiles {
		for hop := range hops {
			r.Paths[name] = append(r.Paths[name], fmt.Sprintf("node%d.%s", hop, r.Profiles[(index+hop)%2]))
		}
	}
	bindReportRuntimes(&r)
	for i, name := range []string{"all-running", "one-client-stopped", "all-restored"} {
		p := phase{Name: name, Required: 25, Barriers: 25, TCPBytes: 32768, UDPBytes: 512, Routes: map[string]routeResult{}}
		for _, profile := range r.Profiles {
			if i != 1 || profile != r.Stopped {
				p.Routes[profile] = routeResult{TCPRounds: 25, UDPRounds: 25}
			}
		}
		p.Profiles, p.Streams = len(p.Routes), 2*len(p.Routes)
		r.Phases = append(r.Phases, p)
	}
	return r
}

func bindReportRuntimes(r *report) {
	for _, engine := range []string{"xray", "sing-box"} {
		for _, role := range []string{"server", "client"} {
			id := role + "/" + engine
			r.Runtimes[id] = Runtime{Path: "/fixture/" + id, ExpectedVersion: "1", VersionOutput: "version 1", SHA256: strings.Repeat("a", 64), VerifiedUnchanged: true}
		}
		r.Assignments.Clients[engine] = "client/" + engine
	}
	for hop := range r.Hops {
		r.Assignments.Nodes[fmt.Sprintf("node%d", hop)] = map[string]string{"xray": "server/xray", "sing-box": "server/sing-box"}
	}
}

func writeReport(t *testing.T, directory string, r report) {
	t.Helper()
	if err := safeio.JSON(filepath.Join(directory, fmt.Sprintf("%d-hop.json", r.Hops)), r); err != nil {
		t.Fatal(err)
	}
}
