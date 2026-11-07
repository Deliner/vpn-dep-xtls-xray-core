package runtimeaudit

import (
	"context"
	"debug/elf"
	"os"
	"testing"
	"vpn-script/internal/node/artifact"
	"vpn-script/internal/runtime/engines"
	"vpn-script/internal/runtime/singbox"
)

func TestPinnedLinuxArchives(t *testing.T) {
	if os.Getenv("CHECK_RUNTIME_ARTIFACTS") != "1" {
		t.Skip("just runtime-artifacts downloads pinned Linux archives")
	}
	audit := runtimeAuditFromEnvironment(t)
	for _, engine := range []string{"xray", "sing-box"} {
		for _, arch := range []string{"x86_64", "aarch64"} {
			t.Run(engine+"/"+arch, func(t *testing.T) { checkPinned(t, engine, arch, audit) })
		}
	}
}

func checkPinned(t *testing.T, engine, arch string, audit *runtimeAudit) {
	t.Helper()
	machine := elf.EM_X86_64
	if arch == "aarch64" {
		machine = elf.EM_AARCH64
	}
	files := pinnedBinaries(t, engine, arch)
	for _, path := range files {
		checkELF(t, path, machine)
	}
	if audit != nil {
		if err := audit.scan(files[engine], engine, arch); err != nil {
			audit.failure(t, engine, arch, err)
		}
	}
}

func pinnedBinaries(t *testing.T, engine, arch string) map[string]string {
	t.Helper()
	raw, err := artifact.Download(context.Background(), engines.Runtime(engine).Assets[arch])
	require(t, err)
	if engine == "xray" {
		path, err := artifact.Extract(raw, t.TempDir())
		require(t, err)
		return map[string]string{"xray": path}
	}
	cpu := "amd64"
	if arch == "aarch64" {
		cpu = "arm64"
	}
	prefix := "sing-box-" + singbox.Version + "-linux-" + cpu + "/"
	files, err := artifact.ExtractTarFiles(raw, t.TempDir(), prefix+"sing-box", prefix+"libcronet.so")
	require(t, err)
	return files
}

func checkELF(t *testing.T, path string, machine elf.Machine) {
	t.Helper()
	binary, err := elf.Open(path)
	require(t, err)
	defer binary.Close()
	if binary.Class != elf.ELFCLASS64 || binary.Machine != machine {
		t.Fatal("incorrect runtime architecture")
	}
}
