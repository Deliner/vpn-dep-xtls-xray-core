package payloadtasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingRuntimeFailsBeforeGoCanSkipTheSuite(t *testing.T) {
	f := newFixture(t)
	args := append(append([]string{}, f.binaries...), "Test", f.report, "", "", "")
	args[2] = filepath.Join(f.root, "missing-client")
	if output, err := f.command("payload.sh", args...).CombinedOutput(); err == nil {
		t.Fatalf("missing runtime became a successful Go invocation: %s", output)
	}
	if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
		t.Fatal("Go ran with an empty/missing binary")
	}
}

func TestPayloadPreservesClientServerAndUpdatedAssignments(t *testing.T) {
	f := newFixture(t)
	args := append(append([]string{}, f.binaries...), "Test", f.report, f.binaries[2], f.binaries[3], "node1,node2")
	if output, err := f.command("payload.sh", args...).CombinedOutput(); err != nil {
		t.Fatalf("payload fixture: %v, %s", err, output)
	}
	trace, err := os.ReadFile(f.trace)
	want := append(append([]string{}, f.binaries...), f.binaries[2], f.binaries[3], "node1,node2")
	if err != nil || string(trace) != strings.Join(want, "\n")+"\n" {
		t.Fatalf("runtime roles changed between recipe and Go: %v, %s", err, trace)
	}
}

func TestPayloadChecksClientRuntimesDespiteNarrowMatrixFilter(t *testing.T) {
	f := newFixture(t)
	args := append(append([]string{}, f.binaries...), "^OneMatrixCase$", f.report, "", "", "")
	if output, err := f.command("payload.sh", args...).CombinedOutput(); err != nil {
		t.Fatalf("payload fixture: %v, %s", err, output)
	}
	trace, err := os.ReadFile(f.trace + ".probe")
	want := strings.Join([]string{f.binaries[2], f.binaries[3], "1"}, "\n") + "\n"
	if err != nil || string(trace) != want {
		t.Fatalf("probe did not use client roles with race support: %q %v", trace, err)
	}
	arguments, err := os.ReadFile(f.trace + ".probe-args")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"-count=1\n", "-race\n", "-run\n^TestPinnedRuntimeIPQuery$\n"} {
		if !strings.Contains(string(arguments), required) {
			t.Errorf("probe lost required option %q: %s", required, arguments)
		}
	}
}

func TestPayloadProbeFailureCannotBecomeAMatrixSuccess(t *testing.T) {
	f := newFixture(t)
	args := append(append([]string{}, f.binaries...), "Test", f.report, "", "", "")
	command := f.command("payload.sh", args...)
	command.Env = append(command.Env, "FAIL_PROBE=1")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("failed real-client gate was ignored: %s", output)
	}
	if _, err := os.Stat(f.trace + ".probe"); err != nil {
		t.Fatalf("probe gate was not reached: %v", err)
	}
	if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
		t.Fatal("matrix ran after client gate failed")
	}
	if _, err := os.Stat(filepath.Join(f.report, "probe.log")); err != nil {
		t.Fatal("failed probe evidence was not retained", err)
	}
}

func TestCompatibilityPreservesFailureAndRunsRemainingStages(t *testing.T) {
	for _, action := range []string{"payload-runtimes", "payload-report"} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t)
			command := f.command("compatibility.sh", f.binaries[2], f.binaries[3], f.report)
			command.Env = append(command.Env, "FAIL_ACTION="+action, "FAIL_STAGE=candidate-clients")
			if output, err := command.CombinedOutput(); err == nil {
				t.Fatalf("failed stage hidden: %s", output)
			}
			assertStatus(t, f.report, "failed")
			for _, stage := range []string{"official-clients", "candidate-clients", "exit-updated", "relay-and-exit-updated"} {
				want := "passed"
				if stage == "candidate-clients" {
					want = "failed"
				}
				assertStatus(t, filepath.Join(f.report, stage), want)
			}
			trace, err := os.ReadFile(f.trace)
			if err != nil || strings.Count(string(trace), "payload-runtimes ") != 4 {
				t.Fatalf("independent stages were skipped: %v, %s", err, trace)
			}
		})
	}
}

func TestCompatibilityRefusesIdenticalCandidateBytes(t *testing.T) {
	f := newFixture(t)
	if output, err := f.command("compatibility.sh", f.binaries[0], f.binaries[3], f.report).CombinedOutput(); err == nil {
		t.Fatalf("identical builds called compatibility coverage: %s", output)
	}
	if _, err := os.Stat(f.report); !os.IsNotExist(err) {
		t.Fatal("invalid comparison created a report run")
	}
}

func assertStatus(t *testing.T, directory, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, "status"))
	if err != nil || string(data) != want+"\n" {
		t.Fatalf("status=%q, expected %q: %v", data, want, err)
	}
}
