package runtimebuild

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTUICCollectorRejectsIncompleteCandidateEvidence(t *testing.T) {
	for _, damage := range []string{"", "missing", "duplicate", "race", "cpu", "changed", "failed"} {
		t.Run(damage, func(t *testing.T) {
			directory := tuicEvidence(t, damage)
			script := `source "$1"; if tuic_verify_candidate "$2" "$2/inventory" arm64; then exit 0; else exit 1; fi`
			command := exec.Command("bash", "-euc", script, "fixture", "../../../ops/runtime/sing-box/tuic/verify.sh", directory)
			output, err := command.CombinedOutput()
			if (err == nil) != (damage == "") {
				t.Fatalf("TUIC evidence acceptance mismatch: %v %s", err, output)
			}
		})
	}
}

func tuicEvidence(t *testing.T, damage string) string {
	t.Helper()
	files := map[string]string{"status": "passed\n", "provenance.txt": "execution_host=linux/arm64\ntuic_patch_state=applied\n",
		"module-verify.log": "all modules verified\n", "inventory": "candidate source inventory\n",
		"candidate-before.sha256": "candidate source inventory\n", "candidate-after.sha256": "candidate source inventory\n"}
	log := "--- PASS: TestServerShutdown (0.20s)\n"
	for _, early := range []bool{false, true} {
		for _, cancel := range []bool{false, true} {
			log += fmt.Sprintf("--- PASS: TestServerShutdown/early=%v/cancel=%v (0.05s)\n", early, cancel)
		}
	}
	files["patched.log"] = strings.Repeat(log, 20)
	damageTUICEvidence(files, damage, log)
	directory := t.TempDir()
	var manifest string
	for name, content := range files {
		write(t, filepath.Join(directory, name), []byte(content), 0600)
		manifest += fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(content)), name)
	}
	write(t, filepath.Join(directory, "evidence.sha256"), []byte(manifest), 0600)
	return directory
}

func damageTUICEvidence(files map[string]string, damage, log string) {
	switch damage {
	case "missing":
		files["patched.log"] = strings.Repeat(log, 19)
	case "duplicate":
		files["patched.log"] += log
	case "race":
		files["patched.log"] += "DATA RACE\n"
	case "cpu":
		files["provenance.txt"] = strings.ReplaceAll(files["provenance.txt"], "arm64", "amd64")
	case "changed":
		files["candidate-after.sha256"] = "changed sources\n"
	case "failed":
		files["status"] = "failed\n"
	}
}

func TestTUICCandidateRejectsUnreviewedDependency(t *testing.T) {
	for _, damage := range []string{"version", "replace", "sum", "directory", "verify", "patch"} {
		t.Run(damage, func(t *testing.T) {
			f := newFixture(t)
			work, cache := tuicFixture(t, f, damage)
			script := `source "$recipe/tuic/prepare.sh"; candidate_tuic_prepare`
			command := exec.Command("bash", "-euc", script)
			command.Env = append(f.command("").Env, "root="+f.root, "work="+work, "GOMODCACHE="+cache,
				"recipe="+filepath.Join(f.root, "ops/runtime/sing-box"), "TUIC_DAMAGE="+damage)
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("unreviewed TUIC dependency accepted: %s", output)
			}
			trace, err := os.ReadFile(f.trace)
			if err != nil || strings.Contains(string(trace), "mod edit") {
				t.Fatalf("unverified source reached module replacement: %v %s", err, trace)
			}
		})
	}
}

func tuicFixture(t *testing.T, f fixture, damage string) (string, string) {
	t.Helper()
	work, cache := filepath.Join(f.root, "work"), filepath.Join(f.root, "cache")
	write(t, filepath.Join(work, "source/go.mod"), []byte("module fixture\n"), 0600)
	module := filepath.Join(cache, "github.com/sagernet/sing-quic@v0.7.2-0.20261020082049-7e0a5869a5f4")
	write(t, filepath.Join(module, "LICENSE"), []byte("fixture source\n"), 0600)
	write(t, filepath.Join(f.root, "tools/go"), []byte(tuicGo), 0700)
	for _, name := range []string{"server-listener", "service-shutdown"} {
		data, err := os.ReadFile("../../../ops/runtime/sing-box/tuic/" + name + ".patch")
		if err != nil {
			t.Fatal(err)
		}
		if damage == "patch" {
			data = append(data, []byte("changed patch\n")...)
		}
		write(t, filepath.Join(f.root, "ops/runtime/sing-box/tuic", name+".patch"), data, 0600)
	}
	return work, cache
}

const tuicGo = `#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FIXTURE_TRACE"
case "$*" in
  *'{{.Version}}'*)
    [[ "$TUIC_DAMAGE" != version ]] || { printf 'unreviewed\n'; exit 0; }
    printf 'v0.7.2-0.20261020082049-7e0a5869a5f4\n' ;;
  *'{{if .Replace}}replaced{{end}}'*)
    if [[ "$TUIC_DAMAGE" == replace ]]; then printf 'replaced\n'; fi ;;
  *'mod download'*) : ;;
  *'mod verify'*) [[ "$TUIC_DAMAGE" != verify ]] ;;
  *'{{.Sum}}'*)
    [[ "$TUIC_DAMAGE" != sum ]] || { printf 'h1:wrong\n'; exit 0; }
    printf 'h1:QuhAb8QW0pKYmSvfj/nK91libunBwVJiWb852yidlxI=\n' ;;
  *'{{.Dir}}'*)
    [[ "$TUIC_DAMAGE" != directory ]] || { printf '/unreviewed/path\n'; exit 0; }
    printf '%s/github.com/sagernet/sing-quic@v0.7.2-0.20261020082049-7e0a5869a5f4\n' "$GOMODCACHE" ;;
  *) exit 42 ;;
esac
`

func TestTUICUpstreamAuditFailureCannotBecomeSuccessfulSourceAudit(t *testing.T) {
	f := auditFixture(t, "sing-box")
	path := filepath.Join(f.root, outputDirectory, "provenance.txt")
	metadata, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, append(metadata, []byte("tuic_patch_state=applied\n")...), 0600)
	helper := "candidate_tuic_audit_upstream() { printf 'upstream finding or failure\\n'; return 17; }\n"
	write(t, filepath.Join(f.root, "ops/runtime/sing-box/tuic/audit.sh"), []byte(helper), 0600)
	output, err := f.command("audit-candidate.sh", outputDirectory).CombinedOutput()
	if err == nil {
		t.Fatalf("upstream failure hidden: %s", output)
	}
	for scan, status := range map[string]int{"source": 17, "binary": 0} {
		data, err := os.ReadFile(filepath.Join(f.root, outputDirectory, "audit", scan+".txt"))
		if err != nil || !strings.HasSuffix(string(data), fmt.Sprintf("scanner_exit_status=%d\n", status)) {
			t.Fatalf("lost %s result: %v %s", scan, err, data)
		}
	}
}
