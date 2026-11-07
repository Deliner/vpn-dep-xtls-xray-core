package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/modules"
	"vpn-script/internal/quality/dependencies/records"
)

func module(name, address string) modules.Module {
	return modules.Module{Path: name, Version: "v1.0.0", Origin: &modules.Origin{
		VCS: "git", URL: address, Hash: strings.Repeat("a", 40)}}
}

func fixture(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	report := modules.Report{Schema: 1}
	for name, directory := range map[string]string{"controller": ".", "xray": "ops/runtime/xray", "sing-box": "ops/runtime/sing-box"} {
		manifest, sums := filepath.Join(directory, "go.mod"), filepath.Join(directory, "go.sum")
		write(t, manifest, "module fixture\n")
		write(t, sums, "checksums")
		manifestSHA, err := safeio.FileHash(manifest)
		if err != nil {
			t.Fatal(err)
		}
		sumsSHA, err := safeio.FileHash(sums)
		if err != nil {
			t.Fatal(err)
		}
		report.Scopes = append(report.Scopes, modules.Scope{Name: name, Directory: directory,
			ManifestSHA: manifestSHA, SumsSHA: sumsSHA, Modules: []modules.Module{module("dep", "https://github.com/o/r")}})
	}
	if err := safeio.PrivateDir("build/dependencies"); err != nil {
		t.Fatal(err)
	}
	if err := safeio.JSON(inventoryPath, report); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func resolvedFixture(t *testing.T) {
	t.Helper()
	var report modules.Report
	if _, err := records.Read("inventory.json", &report); err != nil {
		t.Fatal(err)
	}
	report.Scopes[0].Modules = append(report.Scopes[0].Modules, module("new", "https://github.com/new/repo"),
		module("mirror", "https://source.example/lib"), module("unmapped", "https://source.example/other"))
	if err := records.Save("resolved-inventory.json", report); err != nil {
		t.Fatal(err)
	}
	write(t, "ops/dependencies/mirrors.tsv", "https://source.example/lib\tmirror/repo\t"+strings.Repeat("a", 40)+"\t"+strings.Repeat("b", 40)+"\n")
}
