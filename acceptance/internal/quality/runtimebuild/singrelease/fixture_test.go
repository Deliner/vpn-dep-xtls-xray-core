package singrelease

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"vpn-script/internal/platform/safeio"
)

type fixture struct {
	policy Policy
	input  Inputs
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{input: Inputs{Arch: "arm64", Candidate: filepath.Join(root, "candidate"), Native: filepath.Join(root, "native")}}
	f.policy = Policy{Version: "1.14.4", Label: "nhop-fork-cronet154-zlib-1", Workflow: strings.Repeat("a", 40), Run: "123",
		SourceURL: "https://codeload.github.com/Deliner/vpn-dep-sagernet-sing-box/tar.gz/" + strings.Repeat("b", 40), SourceSHA: strings.Repeat("c", 64)}
	files := map[string][]byte{"source/LICENSE": []byte("license"), "source/go.mod": []byte("module fixture"),
		"source/go.sum": []byte("locks"), "source/main.go": []byte("tested source"),
		"bin/sing-box": []byte("tested Go engine"), "bin/libcronet.so": []byte("tested Cronet")}
	inventory := writeInventory(t, f.input.Candidate, "SHA256SUMS", files)
	provenance := []byte(fmt.Sprintf("engine=sing-box\nbase_version=%s\nbuild_label=%s\ntarget=linux/arm64\nsource_url=%s\nsource_archive_sha256=%s\nloaded_companion_sha256=%s\nbuild_cgo=0\n",
		f.policy.Version, f.policy.Label, f.policy.SourceURL, f.policy.SourceSHA, safeio.Hash(files["bin/libcronet.so"])))
	audit := []byte("No vulnerabilities found.\n\nscanner_exit_status=0\n")
	metadata := map[string][]byte{"status": []byte("built\n"), "provenance.txt": provenance, "audit/provenance.txt": provenance,
		"audit/source.txt": audit, "audit/binary.txt": audit, "audit/actual.sha256": inventory}
	for name, raw := range metadata {
		put(t, f.input.Candidate, name, raw)
	}
	target := Target{Binary: safeio.Hash(files["bin/sing-box"]), Library: safeio.Hash(files["bin/libcronet.so"]),
		Inventory: safeio.Hash(inventory), Provenance: safeio.Hash(provenance), SourceAudit: safeio.Hash(audit), BinaryAudit: safeio.Hash(audit)}
	native := map[string][]byte{"status": []byte("passed\n"), "sing-box-SHA256SUMS": inventory, "sing-box-provenance.txt": provenance,
		"host.txt": []byte("target=linux/arm64\nworkflow_sha=" + f.policy.Workflow + "\nrun_id=" + f.policy.Run + "\n"), "payload/report.json": []byte("reviewed report")}
	target.Evidence = safeio.Hash(writeInventory(t, f.input.Native, "evidence.sha256", native))
	f.policy.Targets = map[string]Target{"amd64": target, "arm64": target}
	return f
}

func writeInventory(t *testing.T, root, name string, files map[string][]byte) []byte {
	t.Helper()
	var names []string
	for path := range files {
		names = append(names, path)
	}
	sort.Slice(names, func(i, j int) bool {
		left, right := strings.HasPrefix(names[i], "source/"), strings.HasPrefix(names[j], "source/")
		if left != right {
			return left
		}
		return names[i] < names[j]
	})
	var manifest strings.Builder
	for _, path := range names {
		put(t, root, path, files[path])
		fmt.Fprintf(&manifest, "%s  %s\n", safeio.Hash(files[path]), path)
	}
	raw := []byte(manifest.String())
	put(t, root, name, raw)
	return raw
}

func put(t *testing.T, root, name string, raw []byte) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
