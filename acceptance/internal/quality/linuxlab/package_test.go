package linuxlab

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func TestPackageRejectsChangedExecutableAndLibrary(t *testing.T) {
	for _, file := range []string{"bin/sing-box", "bin/libcronet.so", "source/source.txt"} {
		t.Run(file, func(t *testing.T) {
			f := newFixture(t)
			write(t, filepath.Join(f.singbox, file), []byte("changed after build"))
			if output, err := f.command().CombinedOutput(); err == nil {
				t.Fatalf("changed candidate accepted: %s", output)
			}
			if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
				t.Fatal("changed candidate reached compilation")
			}
			assertStatus(t, f, "failed\n")
		})
	}
}

func TestPackageRejectsFIFOBeforeCopyingCandidate(t *testing.T) {
	f := newFixture(t)
	if err := syscall.Mkfifo(filepath.Join(f.singbox, "bin/unexpected"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	command := f.command()
	bounded := process.Command(ctx, command.Args[0], command.Args[1:]...)
	bounded.Dir, bounded.Env = command.Dir, command.Env
	output, err := bounded.CombinedOutput()
	_ = process.Stop(bounded)
	if err == nil || !strings.Contains(string(output), "non-regular") {
		t.Fatalf("FIFO reached the copy operation: %v, %s", err, output)
	}
	if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
		t.Fatal("compiler ran with a FIFO in the candidate")
	}
	assertStatus(t, f, "failed\n")
}

func TestPackageRejectsDifferentTarget(t *testing.T) {
	f := newFixture(t)
	write(t, filepath.Join(f.xray, "provenance.txt"), []byte("engine=xray\ntarget=darwin/arm64\n"))
	if output, err := f.command().CombinedOutput(); err == nil {
		t.Fatalf("non-Linux target accepted: %s", output)
	}
	if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
		t.Fatal("wrong target reached compilation")
	}
	assertStatus(t, f, "failed\n")
}

func TestPackagePropagatesCompilerFailure(t *testing.T) {
	f := newFixture(t)
	if output, err := f.command().CombinedOutput(); err == nil {
		t.Fatalf("compiler failure ignored: %s", output)
	}
	if _, err := os.Stat(f.trace); err != nil {
		t.Fatal("verified candidates did not reach compilation", err)
	}
	assertStatus(t, f, "failed\n")
	if _, err := os.Stat(filepath.Join(f.output, "payload.tar.gz")); !os.IsNotExist(err) {
		t.Fatal("failed compilation produced a ready archive")
	}
}

func TestPackagePreservesExistingRun(t *testing.T) {
	f := newFixture(t)
	write(t, filepath.Join(f.output, "status"), []byte("previous evidence\n"))
	if output, err := f.command().CombinedOutput(); err == nil {
		t.Fatalf("existing run replaced: %s", output)
	}
	assertStatus(t, f, "previous evidence\n")
}

func TestPackageRejectsOutputThroughParentSymlink(t *testing.T) {
	f := newFixture(t)
	outside := t.TempDir()
	link := filepath.Join(f.root, "build/link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	f.output = filepath.Join(link, "result")
	if output, err := f.command().CombinedOutput(); err == nil {
		t.Fatalf("outside output accepted: %s", output)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("package wrote outside build through a symlink")
	}
}

func TestInstallerPackageCannotHideCompilerFailureOrReplaceEvidence(t *testing.T) {
	f := newFixture(t)
	if output, err := f.installerCommand().CombinedOutput(); err == nil {
		t.Fatalf("installer compiler failure ignored: %s", output)
	}
	assertStatus(t, f, "failed\n")
	if _, err := os.Stat(f.trace); err != nil {
		t.Fatal("installer compiler was not reached", err)
	}
	if _, err := os.Stat(filepath.Join(f.output, "payload.tar.gz")); !os.IsNotExist(err) {
		t.Fatal("failed compilation produced an executable package")
	}
	write(t, filepath.Join(f.output, "status"), []byte("previous evidence\n"))
	if output, err := f.installerCommand().CombinedOutput(); err == nil {
		t.Fatalf("previous installer run replaced: %s", output)
	}
	assertStatus(t, f, "previous evidence\n")
}

func TestInstallerPackageVerifiesPreviousBinaryBeforeCompilation(t *testing.T) {
	for _, kind := range []string{"valid", "changed", "wrong-cpu", "fifo", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			path, sha := previousInstaller(t, f, kind)
			command := f.installerCommand()
			command.Args = append(command.Args, path, sha)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bounded := process.Command(ctx, command.Args[0], command.Args[1:]...)
			bounded.Dir, bounded.Env = command.Dir, command.Env
			output, runErr := bounded.CombinedOutput()
			_ = process.Stop(bounded)
			if ctx.Err() != nil || runErr == nil {
				t.Fatalf("validation or fixture compiler failure was hidden: %s", output)
			}
			_, err := os.Stat(f.trace)
			if (err == nil) != (kind == "valid") {
				t.Fatalf("unexpected compiler invocation for %s: %v", kind, err)
			}
			assertStatus(t, f, "failed\n")
		})
	}
}
