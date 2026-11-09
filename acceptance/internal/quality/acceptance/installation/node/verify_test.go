package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestTrafficRequiresEveryProtocolDuringEveryLifecyclePhase(t *testing.T) {
	changes := map[string]func(*report){
		"complete":           func(*report) {},
		"missing-phase":      func(r *report) { r.Phases = r.Phases[1:] },
		"relabelled-phase":   func(r *report) { r.Phases[4].Name = "simulated-recovery" },
		"missing-profile":    func(r *report) { r.Profiles = r.Profiles[1:] },
		"duplicate-profile":  func(r *report) { r.Profiles[0] = r.Profiles[1] },
		"serial-traffic":     func(r *report) { r.Phases[4].Streams = 2 },
		"incomplete-barrier": func(r *report) { r.Phases[4].Barriers = 24 },
		"missing-udp":        func(r *report) { r.Phases[4].Routes[r.Profiles[0]] = rounds{TCP: 25} },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			r := fixtureReport()
			change(&r)
			if err := verifyTraffic(r); (err == nil) != (name == "complete") {
				t.Fatalf("traffic evidence mismatch: %v", err)
			}
		})
	}
}

func TestRetainedRuntimeEvidenceRequiresBothRolesAndCapturedBytes(t *testing.T) {
	for _, damage := range []string{"", "missing-role", "wrong-sha", "capture-error", "unverified", "changed-bytes", "wrong-cpu", "wrong-directory"} {
		t.Run(damage, func(t *testing.T) {
			directory, sha, result := fixtureRuntime(t)
			arch, location := "amd64", "previous-runtimes"
			switch damage {
			case "missing-role":
				delete(result.Runtimes, "client/xray")
			case "wrong-sha", "capture-error", "unverified":
				value := result.Runtimes["server/xray"]
				if damage == "wrong-sha" {
					value.SHA = strings.Repeat("b", 64)
				} else if damage == "capture-error" {
					value.Error = "incomplete capture"
				} else {
					value.Verified = false
				}
				result.Runtimes["server/xray"] = value
			case "changed-bytes":
				write(t, filepath.Join(directory, "previous-runtimes/xray"), []byte("changed"))
			case "wrong-cpu":
				arch = "arm64"
			case "wrong-directory":
				location = "runtimes"
			}
			if err := observedRuntimeFiles(directory, location, arch, "xray", sha, result); (err == nil) != (damage == "") {
				t.Fatalf("retained runtime evidence mismatch: %v", err)
			}
		})
	}
}

func TestLifecycleRequiresUpgradeCacheAndAccessEvidence(t *testing.T) {
	path := fixtureReportFile(t, fixtureReport())
	_, err := verifyReport(path)
	require(t, err)
	for _, name := range flags {
		var fields map[string]any
		require(t, readJSON(path, &fields))
		delete(fields, name)
		changed := filepath.Join(t.TempDir(), "incomplete.json")
		require(t, safeio.JSON(changed, fields))
		if _, err := verifyReport(changed); err == nil {
			t.Fatal("missing lifecycle proof accepted", name)
		}
	}
	for _, change := range []func(*report){func(r *report) { r.InstallerSHA = r.PreviousSHA }, func(r *report) { r.WarmRequests = 1 }} {
		r := fixtureReport()
		change(&r)
		if _, err := verifyReport(fixtureReportFile(t, r)); err == nil {
			t.Fatal("invalid installer upgrade/cache claim accepted")
		}
	}
}

func TestCrashProofCannotBeReplacedWithCommittedOrForeignState(t *testing.T) {
	changes := map[string]func(*transaction){
		"complete":             func(*transaction) {},
		"committed":            func(r *transaction) { r.Phase = "committed" },
		"different-release":    func(r *transaction) { r.Next.Release = r.Old.Release },
		"previous-not-running": func(r *transaction) { r.Old.Active = false },
		"wrong-supervisor":     func(r *transaction) { r.Next.State = []byte(`{"files":{"nhop-node":"other"}}`) },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			directory, result := fixtureCrash(t, change)
			if err := verifyCrash(directory, result); (err == nil) != (name == "complete") {
				t.Fatalf("crash evidence mismatch: %v", err)
			}
		})
	}
}

func TestCompleteEvidenceRejectsChangedExtraAndLinkedFiles(t *testing.T) {
	for _, damage := range []string{"", "changed", "extra", "link", "truncated-manifest"} {
		t.Run(damage, func(t *testing.T) {
			directory := fixtureInventory(t)
			switch damage {
			case "changed":
				write(t, filepath.Join(directory, "value"), []byte("changed"))
			case "extra":
				write(t, filepath.Join(directory, "unrecorded"), []byte("other"))
			case "link":
				require(t, os.Symlink("value", filepath.Join(directory, "link")))
			case "truncated-manifest":
				path := filepath.Join(directory, "evidence.sha256")
				raw, err := os.ReadFile(path)
				require(t, err)
				write(t, path, append(raw, []byte(strings.Repeat("x", 100_000))...))
			}
			if err := completeInventory(directory); (err == nil) != (damage == "") {
				t.Fatalf("inventory acceptance mismatch: %v", err)
			}
		})
	}
}

func TestBaselineMustContainActualNSSDNSAndNetworkSnapshots(t *testing.T) {
	raw := ""
	for _, path := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow", "/etc/resolv.conf"} {
		raw += strings.Repeat("a", 64) + "  " + path + "\n"
	}
	raw += "[]\n[]\n[]\n[]\n"
	require(t, baseline([]byte(raw)))
	for _, bad := range []string{"unchanged\n", strings.Replace(raw, "/etc/passwd", "/tmp/foreign", 1), strings.Replace(raw, "[]", "null", 1)} {
		if err := baseline([]byte(bad)); err == nil {
			t.Fatal("missing host state accepted")
		}
	}
}
