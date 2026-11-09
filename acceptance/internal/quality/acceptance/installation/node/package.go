package node

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"vpn-script/internal/platform/safeio"
)

func verifyPackage(directory, snapshot, arch string, result report) error {
	bundle := filepath.Join(directory, "payload")
	inputs, err := manifest(filepath.Join(directory, "SHA256SUMS"))
	if err != nil {
		return err
	}
	if err := verifyFiles(bundle, inputs); err != nil {
		return err
	}
	if err := equalFiles(filepath.Join(directory, "SHA256SUMS"), filepath.Join(bundle, "SHA256SUMS")); err != nil {
		return err
	}
	if inputs["bin/nhop-node"] != result.InstallerSHA || inputs["bin/previous-nhop-node"] != result.PreviousSHA {
		return fmt.Errorf("lifecycle names another installer executable")
	}
	files, err := gzipHashes(filepath.Join(directory, "payload.tar.gz"), "payload/")
	if err != nil {
		return err
	}
	inputs["SHA256SUMS"], err = safeio.FileHash(filepath.Join(directory, "SHA256SUMS"))
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(files, inputs) {
		return fmt.Errorf("delivered package differs from its sealed inputs")
	}
	for _, name := range []string{"nhop-node", "previous-nhop-node"} {
		if err := linuxELF(filepath.Join(bundle, "bin", name), arch); err != nil {
			return err
		}
	}
	return currentSources(bundle, snapshot)
}

func verifyIntegrityLog(directory string) error {
	inputs, err := manifest(filepath.Join(directory, "SHA256SUMS"))
	if err != nil {
		return err
	}
	raw, err := safeio.Read(filepath.Join(directory, "reports/integrity-after.txt"), 1<<20)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		name, ok := strings.CutSuffix(line, ": OK")
		name = strings.TrimPrefix(name, "./")
		if !ok || inputs[name] == "" || seen[name] {
			return fmt.Errorf("invalid post-execution input verification")
		}
		seen[name] = true
	}
	if len(seen) != len(inputs) {
		return fmt.Errorf("incomplete post-execution verification")
	}
	return nil
}
