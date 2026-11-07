package linuxlab

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
)

type fixture struct{ root, xray, singbox, output, trace string }

func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{root: root, output: filepath.Join(root, "build/package"), trace: filepath.Join(root, "trace")}
	for _, script := range []string{"testing/linux/package.sh", "testing/linux/package-common.sh", "testing/linux/installer-package.sh", "runtime/integrity.sh"} {
		data, err := os.ReadFile("../../../ops/" + script)
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, "ops", script), data)
	}
	f.xray = candidate(t, root, "xray")
	f.singbox = candidate(t, root, "sing-box")
	write(t, filepath.Join(root, "tools/go"), []byte("#!/bin/sh\nprintf 'compiler reached\\n' > \"$LAB_TRACE\"\nexit 42\n"))
	if err := os.Chmod(filepath.Join(root, "tools/go"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "build"), 0700); err != nil {
		t.Fatal(err)
	}
	return f
}

func candidate(t *testing.T, root, engine string) string {
	t.Helper()
	path := filepath.Join(root, "candidates", engine)
	files := map[string]string{"source/source.txt": "source", "bin/" + engine: "executable"}
	if engine == "sing-box" {
		files["bin/libcronet.so"] = "companion library"
	}
	write(t, filepath.Join(path, "status"), []byte("built\n"))
	write(t, filepath.Join(path, "provenance.txt"), []byte("engine="+engine+"\ntarget=linux/arm64\n"))
	for name, data := range files {
		write(t, filepath.Join(path, name), []byte(data))
	}
	var names []string
	for name := range files {
		if strings.HasPrefix(name, "bin/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	names = append([]string{"source/source.txt"}, names...)
	var inventory strings.Builder
	for _, name := range names {
		fmt.Fprintf(&inventory, "%x  %s\n", sha256.Sum256([]byte(files[name])), name)
	}
	write(t, filepath.Join(path, "SHA256SUMS"), []byte(inventory.String()))
	return path
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) command() *exec.Cmd {
	command := exec.Command("bash", filepath.Join(f.root, "ops/testing/linux/package.sh"), "arm64", f.xray, f.singbox, f.output)
	command.Dir = f.root
	command.Env = append(os.Environ(), "LAB_TRACE="+f.trace, "PATH="+filepath.Join(f.root, "tools")+":"+os.Getenv("PATH"))
	return command
}

func (f fixture) installerCommand() *exec.Cmd {
	command := f.command()
	command.Args = []string{"bash", filepath.Join(f.root, "ops/testing/linux/installer-package.sh"), "arm64", f.output}
	return command
}

func assertStatus(t *testing.T, f fixture, expected string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.output, "status"))
	if err != nil || string(data) != expected {
		t.Fatalf("status=%q, expected %q: %v", data, expected, err)
	}
}

func previousInstaller(t *testing.T, f fixture, kind string) (string, string) {
	t.Helper()
	header := make([]byte, 20)
	copy(header, []byte{0x7f, 'E', 'L', 'F', 2, 1})
	header[18] = 183 // EM_AARCH64
	if kind == "wrong-cpu" {
		header[18] = 62 // EM_X86_64
	}
	path := filepath.Join(f.root, "previous installer")
	write(t, path, header)
	sha := fmt.Sprintf("%x", sha256.Sum256(header))
	switch kind {
	case "changed":
		write(t, path, append(header, 'x'))
	case "fifo", "symlink":
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		var err error
		if kind == "fifo" {
			err = syscall.Mkfifo(path, 0600)
		} else {
			err = os.Symlink(f.trace, path)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return path, sha
}
