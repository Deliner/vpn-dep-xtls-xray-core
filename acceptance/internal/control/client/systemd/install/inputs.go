package install

import (
	"bytes"
	"debug/elf"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

const binaryLimit = 150_000_000

func loadBundle(options Options, executable []byte) (bundle, error) {
	b := bundle{identity: identity{SchemaVersion: 1, Network: options.Network, Hashes: map[string]string{}}, files: map[string][]byte{}}
	if err := network.ValidateSettings(options.Network); err != nil {
		return b, err
	}
	if options.Network.EngineUID != 0 {
		return b, fmt.Errorf("service identity is resolved at start, not selected at installation")
	}
	value, err := profile.Load(options.Profile)
	if err != nil {
		return b, err
	}
	raw, err := safeio.FormattedJSON(value)
	if err != nil {
		return b, err
	}
	b.files["profile.json"], b.identity.Hashes["profile.json"] = raw, safeio.Hash(raw)
	if err := b.addBinary("nhop-client", executable); err != nil {
		return b, err
	}
	return b, b.loadRuntimes(options.Runtimes)
}

func (b *bundle) loadRuntimes(path string) error {
	var inventory controller.Runtimes
	if err := safeio.LoadJSON(path, &inventory); err != nil {
		return err
	}
	if inventory.SchemaVersion != 1 {
		return fmt.Errorf("unsupported client runtime inventory")
	}
	for name, artifact := range map[string]session.Artifact{"xray": inventory.Xray, "sing-box": inventory.SingBox, "libcronet.so": inventory.Cronet} {
		if !filepath.IsAbs(artifact.Path) || filepath.Clean(artifact.Path) != artifact.Path || !validate.Hash(artifact.SHA256) {
			return fmt.Errorf("%s requires a clean absolute path and trusted SHA-256", name)
		}
		data, err := safeio.Read(artifact.Path, binaryLimit)
		if err != nil {
			return err
		}
		if safeio.Hash(data) != artifact.SHA256 {
			return fmt.Errorf("%s runtime hash mismatch; installation refused", name)
		}
		if err := b.addBinary(name, data); err != nil {
			return err
		}
	}
	return nil
}

func (b *bundle) addBinary(name string, data []byte) error {
	value, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s must be a Linux ELF binary: %w", name, err)
	}
	defer value.Close()
	machine := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}[runtime.GOARCH]
	if machine == 0 || value.Machine != machine || value.Class != elf.ELFCLASS64 || value.Data != elf.ELFDATA2LSB ||
		value.Type != elf.ET_EXEC && value.Type != elf.ET_DYN || name == "libcronet.so" && value.Type != elf.ET_DYN {
		return fmt.Errorf("%s is not an ELF executable/library for Linux %s", name, runtime.GOARCH)
	}
	b.files[name], b.identity.Hashes[name] = data, safeio.Hash(data)
	return nil
}

// /proc/self/exe intentionally follows the kernel's reference to this running
// executable, not a possibly replaced pathname returned by os.Executable.
func runningExecutable() ([]byte, error) {
	file, err := os.Open("/proc/self/exe")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, binaryLimit+1))
	if len(data) > binaryLimit {
		return nil, fmt.Errorf("client executable exceeds size limit")
	}
	return data, err
}
