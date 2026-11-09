package release

import (
	"debug/elf"
	"fmt"
	"path/filepath"
	"strings"
	"vpn-script/internal/platform/safeio"
)

// Verify binds both native executions and complete retained inputs to a reviewed
// policy. It neither executes downloaded libraries nor changes installer pins.
func Verify(policy Policy, collection string) error {
	if err := policy.valid(); err != nil {
		return err
	}
	for _, arch := range []string{"amd64", "arm64"} {
		if err := verifyTarget(policy, collection, arch); err != nil {
			return fmt.Errorf("Cronet %s: %w", arch, err)
		}
	}
	return nil
}

func verifyTarget(policy Policy, collection, arch string) error {
	target := policy.Targets[arch]
	build, native := filepath.Join(collection, "cronet-linux-"+arch), filepath.Join(collection, "cronet-native-"+arch)
	inputs, err := inventory(build, target.Build, nil)
	if err != nil {
		return err
	}
	run, err := inventory(native, target.Native, map[string]string{"naive/evidence.sha256": target.Naive, "zlib-window/evidence.sha256": target.Zlib})
	if err != nil {
		return err
	}
	if inputs["bin/libcronet.so"] != target.Library || inputs["source.tsv"] != policy.Sources || run["naive/bin/libcronet.so"] != target.Library || run["naive/cronet-source.tsv"] != policy.Sources {
		return fmt.Errorf("native execution did not use reviewed library and sources")
	}
	for path, want := range map[string]string{filepath.Join(build, "status"): "native-built\n", filepath.Join(native, "status"): "passed\n", filepath.Join(native, "naive/status"): "passed\n", filepath.Join(native, "zlib-window/status"): "passed\n"} {
		raw, err := safeio.Read(path, 1024)
		if err != nil || string(raw) != want {
			return fmt.Errorf("unsuccessful native evidence: %s", path)
		}
	}
	identity, err := safeio.Read(filepath.Join(native, "identity.txt"), 65536)
	if err != nil {
		return err
	}
	for _, line := range []string{"workflow_sha=" + policy.Workflow, "run_id=" + policy.Run} {
		if strings.Count("\n"+string(identity), "\n"+line+"\n") != 1 {
			return fmt.Errorf("native workflow/run identity differs")
		}
	}
	return libraryCPU(filepath.Join(build, "bin/libcronet.so"), arch)
}

func libraryCPU(path, arch string) error {
	file, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	want := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}[arch]
	if file.Class != elf.ELFCLASS64 || file.Data != elf.ELFDATA2LSB || file.Type != elf.ET_DYN || file.Machine != want {
		return fmt.Errorf("wrong native shared-library architecture")
	}
	return nil
}
