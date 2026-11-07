package owned

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func TestOwnedModuleRejectsChangedExtractedSourceAndGoMod(t *testing.T) {
	for _, changed := range []string{"none", "source", "mod", "zip"} {
		t.Run(changed, func(t *testing.T) {
			module := contentFixture(t)
			target := map[string]string{"source": filepath.Join(module.Dir, "value.go"), "mod": module.GoMod, "zip": module.Zip}[changed]
			if target != "" {
				if err := os.WriteFile(target, []byte("modified"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err := verifyContent(module)
			if (err != nil) != (changed != "none") {
				t.Fatalf("wrong content decision %s: %v", changed, err)
			}
		})
	}
}

func contentFixture(t *testing.T) Module {
	t.Helper()
	directory := t.TempDir()
	module := Module{Path: "github.com/Deliner/vpn-dep-fixture", Version: "v1.0.0", Dir: filepath.Join(directory, "source"), GoMod: filepath.Join(directory, "module.mod"), Zip: filepath.Join(directory, "module.zip")}
	files := map[string]string{"value.go": "package fixture\n", "go.mod": "module github.com/upstream/fixture\n"}
	writeFixtureContent(t, module, files)
	var err error
	module.Sum, err = dirhash.HashZip(module.Zip, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}
	module.GoModSum, err = dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(files["go.mod"])), nil })
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func writeFixtureContent(t *testing.T, module Module, files map[string]string) {
	t.Helper()
	if err := os.Mkdir(module.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, contents := range files {
		entry, err := writer.Create(module.Path + "@" + module.Version + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(module.Dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(module.Zip, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(module.GoMod, []byte(files["go.mod"]), 0600); err != nil {
		t.Fatal(err)
	}
}
