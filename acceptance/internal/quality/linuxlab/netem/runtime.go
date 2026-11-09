package netem

import (
	"debug/buildinfo"
	"debug/elf"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"vpn-script/internal/platform/safeio"
)

func nativeIdentity(root, arch, commit, snapshot string) error {
	identity, err := properties(filepath.Join(root, "identity.txt"))
	if err != nil {
		return err
	}
	machine := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[arch]
	if machine == "" || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(commit) || identity["workflow_sha"] != commit ||
		identity["target"] != "linux/"+arch || identity["machine"] != machine || !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(identity["run_id"]) {
		return fmt.Errorf("native execution identity differs")
	}
	if err := equalFiles(filepath.Join(root, "source.txt"), filepath.Join(snapshot, "source.txt")); err != nil {
		return err
	}
	if err := equalFiles(filepath.Join(root, "host-before.txt"), filepath.Join(root, "host-after.txt")); err != nil {
		return err
	}
	return nativeExecutable(filepath.Join(root, "network.test"), arch)
}

func nativeExecutable(path, arch string) error {
	file, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	want := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}[arch]
	if file.Class != elf.ELFCLASS64 || file.Data != elf.ELFDATA2LSB || file.Machine != want {
		return fmt.Errorf("wrong native binary: %s", path)
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return err
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if info.GoVersion != "go1.27.3" || settings["GOOS"] != "linux" || settings["GOARCH"] != arch || settings["CGO_ENABLED"] != "0" {
		return fmt.Errorf("native Go build identity differs: %s", path)
	}
	return nil
}

func verifyRuntime(root, arch, snapshot string) error {
	runtime := filepath.Join(root, "runtime")
	if err := textFile(filepath.Join(runtime, "status"), "built\n"); err != nil {
		return err
	}
	manifest, err := readManifest(filepath.Join(runtime, "SHA256SUMS"))
	if err != nil {
		return err
	}
	for _, name := range []string{"source", "bin"} {
		files, err := regularFiles(filepath.Join(runtime, name))
		if err != nil {
			return err
		}
		for path := range files {
			key := name + "/" + path
			actual, err := safeio.FileHash(filepath.Join(runtime, key))
			if err != nil || manifest[key] != actual {
				return fmt.Errorf("runtime source/bin inventory differs: %s", key)
			}
			delete(manifest, key)
		}
	}
	if len(manifest) != 0 {
		return fmt.Errorf("runtime inventory has missing files")
	}
	for _, name := range []string{"runtime-observed.sha256", "runtime-after.sha256"} {
		if err := equalFiles(filepath.Join(root, name), filepath.Join(runtime, "SHA256SUMS")); err != nil {
			return err
		}
	}
	return pinnedRuntime(runtime, arch, snapshot)
}

func pinnedRuntime(runtime, arch, snapshot string) error {
	pins, err := safeio.Read(filepath.Join(snapshot, "ops/testing/acceptance/masque/runtimes.tsv"), 4096)
	if err != nil {
		return err
	}
	actual, err := safeio.FileHash(filepath.Join(runtime, "bin/xray"))
	if err != nil {
		return err
	}
	if strings.Count(string(pins), "\n"+arch+"\t"+actual+"\n") != 1 {
		return fmt.Errorf("runtime differs from the reviewed ordinary candidate")
	}
	provenance, err := safeio.Read(filepath.Join(runtime, "provenance.txt"), 1<<20)
	if err != nil {
		return err
	}
	for _, value := range []string{"engine=xray\n", "target=linux/" + arch + "\n", "build_label=nhop-fork-masque-sack-1\n"} {
		if strings.Count(string(provenance), value) != 1 {
			return fmt.Errorf("runtime provenance differs: %s", value)
		}
	}
	return nativeExecutable(filepath.Join(runtime, "bin/xray"), arch)
}
