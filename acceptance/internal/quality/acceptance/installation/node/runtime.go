package node

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/integration/evidence/installation/previous"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/singbox"
	"vpn-script/internal/runtime/xray"
)

func verifyRuntimes(directory, arch string, result report) error {
	machine := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[arch]
	pins := map[string]model.Asset{"xray": xray.Runtime().Assets[machine], "sing-box": singbox.Runtime().Assets[machine]}
	if len(result.Runtimes) != 4 || len(result.PreviousRuntimes) != 4 {
		return fmt.Errorf("incomplete published runtime identity")
	}
	for _, name := range []string{"xray", "sing-box"} {
		historical, err := previous.Runtime(name)
		if err != nil {
			return err
		}
		if old := historical.Assets[machine]; old.SHA256 != pins[name].SHA256 {
			pins["previous/"+name] = old
		}
	}
	if len(result.Archives) != len(pins) {
		return fmt.Errorf("missing or extra runtime archive generation")
	}
	if err := verifyArchives(directory, pins, result); err != nil {
		return err
	}
	if err := verifyXray(directory, arch, pins["xray"], result); err != nil {
		return err
	}
	if err := verifySingbox(directory, arch, pins["sing-box"], result); err != nil {
		return err
	}
	return verifyPreviousRuntimes(directory, arch, machine, result)
}

func verifyArchives(directory string, pins map[string]model.Asset, result report) error {
	var size int64
	for name, asset := range pins {
		if result.Archives[name] != asset.SHA256 {
			return fmt.Errorf("unreviewed installed runtime archive: %s", name)
		}
		path := filepath.Join(directory, "archives", asset.SHA256+".archive")
		info, err := os.Stat(path)
		if err != nil || info.Size() != asset.Size {
			return fmt.Errorf("runtime archive size differs: %s", name)
		}
		if err := verifyFiles(filepath.Dir(path), map[string]string{filepath.Base(path): asset.SHA256}); err != nil {
			return err
		}
		size += asset.Size
	}
	if result.CachedBytes != size {
		return fmt.Errorf("current and retained runtime cache size differs")
	}
	return nil
}

func verifyPreviousRuntimes(directory, arch, machine string, result report) error {
	changed := false
	for _, name := range []string{"xray", "sing-box"} {
		runtime, err := previous.Runtime(name)
		if err != nil {
			return err
		}
		archive := filepath.Join(directory, "archives", runtime.Assets[machine].SHA256+".archive")
		sha, err := previousBinary(directory, archive, arch, name, runtime.Version)
		if err != nil {
			return err
		}
		prior := result
		prior.Runtimes = result.PreviousRuntimes
		if err := observedRuntimeFiles(directory, "previous-runtimes", arch, name, sha, prior); err != nil {
			return err
		}
		changed = changed || sha != result.Runtimes["server/"+name].SHA
	}
	if !changed {
		return fmt.Errorf("no old-to-new runtime transition was proven")
	}
	return nil
}

func previousBinary(directory, archive, arch, engine, version string) (string, error) {
	if engine == "xray" {
		return zipHash(archive, "xray")
	}
	files, err := gzipHashes(archive, "sing-box-"+version+"-linux-"+arch+"/")
	if err != nil {
		return "", err
	}
	if err := verifyFiles(filepath.Join(directory, "previous-runtimes"), map[string]string{"libcronet.so": files["libcronet.so"]}); err != nil {
		return "", err
	}
	return files["sing-box"], linuxELF(filepath.Join(directory, "previous-runtimes/libcronet.so"), arch)
}

func verifyXray(directory, arch string, asset model.Asset, result report) error {
	sha, err := zipHash(filepath.Join(directory, "archives", asset.SHA256+".archive"), "xray")
	if err != nil {
		return err
	}
	return observedRuntime(directory, arch, "xray", sha, result)
}

func verifySingbox(directory, arch string, asset model.Asset, result report) error {
	prefix := "sing-box-" + singbox.Version + "-linux-" + arch + "/"
	files, err := gzipHashes(filepath.Join(directory, "archives", asset.SHA256+".archive"), prefix)
	if err != nil {
		return err
	}
	if err := observedRuntime(directory, arch, "sing-box", files["sing-box"], result); err != nil {
		return err
	}
	if err := verifyFiles(filepath.Join(directory, "runtimes"), map[string]string{"libcronet.so": files["libcronet.so"]}); err != nil {
		return err
	}
	return linuxELF(filepath.Join(directory, "runtimes", "libcronet.so"), arch)
}

func observedRuntime(directory, arch, name, sha string, result report) error {
	return observedRuntimeFiles(directory, "runtimes", arch, name, sha, result)
}

func observedRuntimeFiles(directory, files, arch, name, sha string, result report) error {
	for _, role := range []string{"client/", "server/"} {
		observed, exists := result.Runtimes[role+name]
		if !exists || !observed.Verified || observed.Error != "" || observed.SHA != sha {
			return fmt.Errorf("tested %s%s differs from its published archive", role, name)
		}
	}
	path := filepath.Join(directory, files, name)
	actual, err := safeio.FileHash(path)
	if err != nil || actual != sha {
		return fmt.Errorf("captured runtime differs from its archive: %s", name)
	}
	return linuxELF(path, arch)
}
