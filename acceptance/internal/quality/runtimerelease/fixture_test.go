package runtimerelease

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type fixture struct {
	policy Policy
	input  Inputs
	output string
	root   string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{root: root, output: filepath.Join(root, "output"),
		input: Inputs{Candidate: filepath.Join(root, "candidate"), Native: filepath.Join(root, "native"), Arch: "arm64"}}
	f.policy = Policy{Version: "26.10.31", Label: "nhop-fork-fixture-1", WorkflowSHA: strings.Repeat("a", 40),
		SourceURL:    "https://codeload.github.com/Deliner/vpn-dep-xtls-xray-core/tar.gz/" + strings.Repeat("b", 40),
		SourceSHA256: strings.Repeat("c", 64), RunURL: "https://github.com/Deliner/vpn-dep-xtls-xray-core/actions/runs/42"}
	files := map[string][]byte{"bin/xray": []byte("tested executable"), "source/LICENSE": []byte("license"),
		"source/go.mod": []byte("module fixture"), "source/go.sum": []byte("lock"), "source/core/start.go": []byte("source")}
	manifest := writeInventory(t, f.input.Candidate, "SHA256SUMS", files)
	f.policy.Targets = map[string]Target{"arm64": {BinarySHA256: hash(files["bin/xray"]), InventorySHA256: hash(manifest)}}
	f.candidateMetadata(t, manifest)
	f.sealNative(t, manifest)
	return f
}

func (f fixture) candidateMetadata(t *testing.T, manifest []byte) {
	t.Helper()
	p := f.policy
	text := fmt.Sprintf("engine=xray\nbase_version=%s\nbuild_label=%s\ntarget=linux/arm64\nsource_url=%s\nsource_archive_sha256=%s\n",
		p.Version, p.Label, p.SourceURL, p.SourceSHA256)
	files := map[string][]byte{"status": []byte("built\n"), "provenance.txt": []byte(text),
		"audit/provenance.txt": []byte(text), "audit/actual.sha256": manifest,
		"audit/source.txt": []byte("No vulnerabilities found.\nscanner_exit_status=0\n"),
		"audit/binary.txt": []byte("No vulnerabilities found.\nscanner_exit_status=0\n")}
	for name, raw := range files {
		write(t, filepath.Join(f.input.Candidate, name), raw)
	}
}

func (f *fixture) sealNative(t *testing.T, candidate []byte) {
	t.Helper()
	files := map[string][]byte{"xray-SHA256SUMS": candidate, "status": []byte("passed\n"),
		"host.txt":           []byte("target=linux/arm64\nworkflow_sha=" + f.policy.WorkflowSHA + "\n"),
		"payload/1-hop.json": []byte("reviewed evidence")}
	manifest := writeInventory(t, f.input.Native, "evidence.sha256", files)
	target := f.policy.Targets["arm64"]
	target.EvidenceSHA256 = hash(manifest)
	f.policy.Targets["arm64"], f.policy.Targets["amd64"] = target, target
}

func writeInventory(t *testing.T, root, name string, files map[string][]byte) []byte {
	t.Helper()
	var names, lines []string
	for path := range files {
		if path != "bin/xray" {
			names = append(names, path)
		}
	}
	sort.Strings(names)
	if _, ok := files["bin/xray"]; ok {
		names = append(names, "bin/xray")
	}
	for _, path := range names {
		write(t, filepath.Join(root, path), files[path])
		lines = append(lines, fmt.Sprintf("%s  %s\n", hash(files[path]), path))
	}
	manifest := []byte(strings.Join(lines, ""))
	write(t, filepath.Join(root, name), manifest)
	return manifest
}

func write(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
