package runtimebuild

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type fixture struct {
	root    string
	archive string
	trace   string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{root: root}
	f.archive, f.trace = filepath.Join(f.root, "fixture.tar.gz"), filepath.Join(f.root, "trace")
	for _, script := range []string{"candidate.sh", "audit-candidate.sh", "integrity.sh", "compare.sh", "xray/build.sh", "xray/ephemeral-ports.patch", "sing-box/build.sh", "sing-box/cronet.sh", "sing-box/native.sh"} {
		data, err := os.ReadFile("../../../ops/runtime/" + script)
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(f.root, "ops/runtime", script), data, 0600)
	}
	f.source(t, "\t\t\t\t}\n\t\t\t}\n\n\t\t\tsetReusePort(fd)\n\t\t})\n\t}\n}\n")
	write(t, filepath.Join(f.root, "ops/runtime/xray/go.mod"), []byte("module fixture\n"), 0600)
	write(t, filepath.Join(f.root, "ops/runtime/xray/go.sum"), nil, 0600)
	write(t, filepath.Join(f.root, "tools/go"), []byte(fakeGo), 0700)
	write(t, filepath.Join(f.root, "tools/curl"), []byte(fakeCurl), 0700)
	return f
}

func sourceArchive(t *testing.T, path, listener string) []byte {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	for name, body := range map[string]string{"source.txt": "verified upstream contents\n", "transport/internet/system_listener.go": listener} {
		if err := archive.WriteHeader(&tar.Header{Name: "upstream/" + name, Mode: 0644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, close := range []func() error{archive.Close, compressed.Close, file.Close} {
		if err := close(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (f fixture) source(t *testing.T, listener string) {
	t.Helper()
	archive := sourceArchive(t, f.archive, listener)
	pin := fmt.Sprintf("26.10.31\thttps://example.invalid/source\t%x\t%d\n", sha256.Sum256(archive), len(archive))
	write(t, filepath.Join(f.root, "ops/runtime/xray/source.tsv"), []byte(pin), 0600)
}

func write(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) command(script string, arguments ...string) *exec.Cmd {
	command := exec.Command("bash", append([]string{filepath.Join(f.root, "ops/runtime", script)}, arguments...)...)
	command.Dir = f.root
	command.Env = append(os.Environ(), "PATH="+filepath.Join(f.root, "tools")+":"+os.Getenv("PATH"),
		"FIXTURE_ARCHIVE="+f.archive, "FIXTURE_TRACE="+f.trace)
	return command
}

const fakeGo = `#!/usr/bin/env bash
if [[ "$1" == env && "$2" == GOVERSION ]]; then printf 'go1.27.3\n'; exit 0; fi
printf '%s\n' "$*" >> "$FIXTURE_TRACE"
exit 42
`

const fakeCurl = `#!/usr/bin/env bash
while (($#)); do
  if [[ "$1" == --output ]]; then cat "$FIXTURE_ARCHIVE" > "$2"; exit 0; fi
  shift
done
exit 2
`
