package node

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/singbox"
	"vpn-script/internal/runtime/xray"
)

func verifyRuntimes(directory, arch string, result report) error {
	machine := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[arch]
	pins := map[string]model.Asset{"xray": xray.Runtime().Assets[machine], "sing-box": singbox.Runtime().Assets[machine]}
	if len(result.Archives) != 2 || len(result.Runtimes) != 4 || result.CachedBytes != pins["xray"].Size+pins["sing-box"].Size {
		return fmt.Errorf("incomplete published runtime identity")
	}
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
	}
	if err := verifyXray(directory, arch, pins["xray"], result); err != nil {
		return err
	}
	return verifySingbox(directory, arch, pins["sing-box"], result)
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
	for _, role := range []string{"client/", "server/"} {
		observed, exists := result.Runtimes[role+name]
		if !exists || !observed.Verified || observed.Error != "" || observed.SHA != sha {
			return fmt.Errorf("tested %s%s differs from its published archive", role, name)
		}
	}
	path := filepath.Join(directory, "runtimes", name)
	actual, err := safeio.FileHash(path)
	if err != nil || actual != sha {
		return fmt.Errorf("captured runtime differs from its archive: %s", name)
	}
	return linuxELF(path, arch)
}
