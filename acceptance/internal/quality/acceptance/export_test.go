package acceptance

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestSourceExportBuildsEmbeddedViewsAndExcludesPrivateInputs(t *testing.T) {
	root := exportFixture(t)
	archive := filepath.Join(root, "build", "source.tar.gz")
	command := exec.Command("bash", filepath.Join(root, "ops/release/export-public.sh"), archive)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("export: %v %s", err, output)
	}
	files := exportedFiles(t, archive)
	for _, name := range []string{"internal/ui/views/page.html", "internal/ui/views/style.css", "ops/release/runtime/xray.json",
		"ops/release/runtime/cronet.json", "ops/release/runtime/sing-box.json",
		"ops/testing/acceptance/installer/workflow.yml", "ops/testing/cronet/zlib/window.c", "ops/testing/cronet/zlib/window.patch"} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || files[name] != string(raw) || !strings.Contains(files["ops/release/SHA256SUMS"], safeio.Hash(raw)+"  vpn-script/"+name+"\n") {
			t.Fatalf("maintained build input missing or changed: %s", name)
		}
	}
	for _, name := range []string{"config.env", "build/private.html", ".ssh-custom/key.go", "docs/private.local.md", "ops/release/runtime/private.json"} {
		if _, found := files[name]; found {
			t.Fatalf("private input exported: %s", name)
		}
	}
	unpacked := t.TempDir()
	for name, raw := range files {
		write(t, filepath.Join(unpacked, name), raw, 0600)
	}
	command = exec.Command("go", "build", "./...")
	command.Dir = unpacked
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("export cannot compile embedded assets: %v %s", err, output)
	}
}

func exportFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{".gitignore", "AGENTS.md", "README.md", "go.sum", "justfile", ".github/workflows/go.yml",
		"ops/release/runtime/xray.json", "ops/release/runtime/cronet.json", "ops/release/runtime/sing-box.json", "ops/release/runtime/private.json",
		"ops/testing/acceptance/installer/workflow.yml",
		"ops/testing/cronet/zlib/window.c", "ops/testing/cronet/zlib/window.patch",
		"ops/runtime/inputs.tsv", "docs/public.md", "examples/example.yaml", "config.env", "build/private.html", ".ssh-custom/key.go", "docs/private.local.md"} {
		write(t, filepath.Join(root, name), "fixture "+name+"\n", 0600)
	}
	write(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.27\n", 0600)
	write(t, filepath.Join(root, "go.sum"), "", 0600)
	write(t, filepath.Join(root, "cmd/main.go"), "package main\nfunc main() {}\n", 0600)
	write(t, filepath.Join(root, "internal/ui/assets.go"), "package ui\nimport _ \"embed\"\n//go:embed views/page.html\nvar Page string\n//go:embed views/style.css\nvar CSS string\n", 0600)
	write(t, filepath.Join(root, "internal/ui/views/page.html"), "<main>Fixture</main>\n", 0600)
	write(t, filepath.Join(root, "internal/ui/views/style.css"), "main { color: navy; }\n", 0600)
	raw, err := os.ReadFile("../../../ops/release/export-public.sh")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "ops/release/export-public.sh"), string(raw), 0700)
	return root
}

func exportedFiles(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	files := map[string]string{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return files
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		name := strings.TrimPrefix(header.Name, "vpn-script/")
		if header.Typeflag != tar.TypeReg || name == header.Name || !filepath.IsLocal(name) {
			t.Fatalf("unexpected archive member %q", header.Name)
		}
		raw, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(raw)
	}
}
