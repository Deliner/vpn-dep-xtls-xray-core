package evidence

import (
	"runtime"
	"testing"
	"time"
)

func TestReportVerificationRequiresEveryRequestedTopology(t *testing.T) {
	directory := t.TempDir()
	since := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
	writeReport(t, directory, successfulReport(1, since))
	if err := CheckReports(directory, []int{1, 3}, []string{"a", "b"}, since, ""); err == nil {
		t.Fatal("missing three-hop report accepted")
	}
	writeReport(t, directory, successfulReport(3, since))
	if err := CheckReports(directory, []int{1, 3}, []string{"a", "b"}, since, ""); err != nil {
		t.Fatal(err)
	}
	if err := CheckReports(directory, []int{1, 3}, []string{"a", "b"}, since.Add(time.Second), ""); err == nil {
		t.Fatal("old successful reports accepted for a new run")
	}
	for _, hops := range [][]int{nil, {0}, {2}, {1, 1}} {
		if err := CheckReports(directory, hops, []string{"a", "b"}, since, ""); err == nil {
			t.Fatalf("ambiguous topology request accepted: %v", hops)
		}
	}
}

func TestIncompleteReportsCannotClaimFullSuccess(t *testing.T) {
	cases := []struct {
		name   string
		change func(*report)
	}{
		{"old schema", func(r *report) { r.SchemaVersion = 3 }},
		{"skipped", func(r *report) { r.Status = "skipped" }},
		{"other platform", func(r *report) { r.Platform = "other/other" }},
		{"missing phase", func(r *report) { r.Phases = r.Phases[:2] }},
		{"duplicate profile", func(r *report) { r.Profiles[1] = "a" }},
		{"repeated service", func(r *report) { r.Paths["b"] = r.Paths["a"] }},
		{"unknown stopped client", func(r *report) { r.Stopped = "missing" }},
		{"missing barrier", func(r *report) { r.Phases[0].Barriers = 24 }},
		{"wrong streams", func(r *report) { r.Phases[0].Streams-- }},
		{"tiny packets", func(r *report) { r.Phases[0].TCPBytes = 1 }},
		{"missing arrivals", func(r *report) { r.Phases[0].Missing = []string{"a/udp"} }},
		{"incomplete UDP", func(r *report) { r.Phases[0].Routes["a"] = routeResult{TCPRounds: 25, UDPRounds: 24} }},
		{"failed route", func(r *report) { r.Phases[0].Routes["a"] = routeResult{TCPRounds: 25, UDPRounds: 25, Error: "failed"} }},
		{"missing runtime", func(r *report) { delete(r.Runtimes, "client/xray") }},
		{"missing node binding", func(r *report) { delete(r.Assignments.Nodes["node2"], "xray") }},
		{"missing client binding", func(r *report) { delete(r.Assignments.Clients, "xray") }},
		{"unused identity", func(r *report) { r.Runtimes["unused"] = r.Runtimes["client/xray"] }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			since := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
			r := successfulReport(3, since)
			test.change(&r)
			if err := r.check(3, []string{"a", "b"}, since, runtime.GOOS+"/"+runtime.GOARCH); err == nil {
				t.Fatal("incomplete evidence accepted")
			}
		})
	}
}

func TestReportAddressesCannotSubstituteAnotherIPFamily(t *testing.T) {
	directory := t.TempDir()
	since := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
	for _, addresses := range [][]string{{"::1"}, {"::1", "::1", "::1"}, {"::1", "127.0.0.1", "::1"}, {"127.0.0.1", "::1", "127.0.0.1"}} {
		r := successfulReport(len(addresses), since)
		r.NodeAddresses = addresses
		writeReport(t, directory, r)
		if err := CheckReports(directory, []int{r.Hops}, r.Profiles, since, "", addresses...); err != nil {
			t.Fatal(err)
		}
		if CheckReports(directory, []int{r.Hops}, r.Profiles, since, "") == nil {
			t.Fatal("IPv4 expectation accepted an IPv6 hop")
		}
		r.NodeAddresses = nil
		writeReport(t, directory, r)
		if CheckReports(directory, []int{r.Hops}, r.Profiles, since, "", addresses...) == nil {
			t.Fatal("report without node addresses accepted")
		}
	}
	r := successfulReport(3, since)
	writeReport(t, directory, r)
	for _, addresses := range [][]string{{"::1"}, {"127.0.0.1", "::1", "127.0.0.1"}, {"localhost"}, {"127.0.0.1", "127.0.0.1"}} {
		if CheckReports(directory, []int{3}, r.Profiles, since, "", addresses...) == nil {
			t.Fatalf("wrong or ambiguous expectation accepted: %v", addresses)
		}
	}
}

func TestRuntimeIdentityMustRemainVerified(t *testing.T) {
	for _, change := range []func(*Runtime){
		func(r *Runtime) { r.VerifiedUnchanged = false }, func(r *Runtime) { r.Error = "changed" },
		func(r *Runtime) { r.SHA256 = "not a digest" }, func(r *Runtime) { r.Path = "relative" },
		func(r *Runtime) { r.VersionOutput = "" }, func(r *Runtime) { r.ExpectedVersion = "" },
	} {
		since := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
		r := successfulReport(1, since)
		identity := r.Runtimes["client/xray"]
		change(&identity)
		r.Runtimes["client/xray"] = identity
		if err := r.check(1, []string{"a", "b"}, since, runtime.GOOS+"/"+runtime.GOARCH); err == nil {
			t.Fatal("invalid executable identity accepted")
		}
	}
}

func TestRuntimeAssignmentsCannotSwapEnginesOrClientServerRoles(t *testing.T) {
	for _, change := range []func(*report){
		func(r *report) {
			r.Assignments.Clients["xray"], r.Assignments.Clients["sing-box"] = "client/sing-box", "client/xray"
		},
		func(r *report) {
			r.Assignments.Clients, r.Assignments.Nodes["node0"] = r.Assignments.Nodes["node0"], r.Assignments.Clients
		},
	} {
		since := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
		r := successfulReport(1, since)
		change(&r)
		if err := r.check(1, []string{"a", "b"}, since, runtime.GOOS+"/"+runtime.GOARCH); err == nil {
			t.Fatal("swapped runtime assignments accepted")
		}
	}
}

func TestExplicitReportPlatformDoesNotRelaxEvidence(t *testing.T) {
	directory := t.TempDir()
	since := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
	for _, platform := range []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"} {
		r := successfulReport(1, since)
		r.Platform = platform
		writeReport(t, directory, r)
		if err := CheckReports(directory, []int{1}, r.Profiles, since, platform); err != nil {
			t.Fatalf("explicit producer %s: %v", platform, err)
		}
		if platform != runtime.GOOS+"/"+runtime.GOARCH && CheckReports(directory, []int{1}, r.Profiles, since, "") == nil {
			t.Fatal("default verifier silently accepted another platform")
		}
		r.Phases[0].Barriers--
		writeReport(t, directory, r)
		if CheckReports(directory, []int{1}, r.Profiles, since, platform) == nil {
			t.Fatal("explicit platform bypassed incomplete traffic evidence")
		}
	}
	for _, platform := range []string{"windows/amd64", "linux/arm", "other/other"} {
		r := successfulReport(1, since)
		r.Platform = platform
		writeReport(t, directory, r)
		if CheckReports(directory, []int{1}, r.Profiles, since, platform) == nil {
			t.Fatal("unsupported producer platform accepted")
		}
	}
}
