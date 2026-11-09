package runtimerelease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"vpn-script/internal/platform/safeio"
)

func Stage(policy Policy, amd64, arm64, output string) (err error) {
	if err := policy.validate(); err != nil {
		return err
	}
	files := stageIdentity(policy)
	for arch, input := range map[string]string{"amd64": amd64, "arm64": arm64} {
		value, raw, err := packaged(policy, input, arch, "receipt.json")
		if err != nil {
			return err
		}
		metadata, _ := json.MarshalIndent(value, "", "  ")
		files[value.Archive], files["xray-"+arch+".json"] = raw, append(metadata, '\n')
	}
	if err := os.Mkdir(output, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(output)
		}
	}()
	for name, raw := range files {
		if err := safeio.Create(filepath.Join(output, name), raw, 0600); err != nil {
			return err
		}
	}
	return safeio.Create(filepath.Join(output, "SHA256SUMS"), manifest(files), 0600)
}

func VerifyStage(policy Policy, directory string) error {
	if err := policy.validate(); err != nil {
		return err
	}
	files := stageIdentity(policy)
	for _, arch := range []string{"amd64", "arm64"} {
		value, raw, err := packaged(policy, directory, arch, "xray-"+arch+".json")
		if err != nil {
			return err
		}
		metadata, _ := json.MarshalIndent(value, "", "  ")
		files[value.Archive], files["xray-"+arch+".json"] = raw, append(metadata, '\n')
	}
	files["SHA256SUMS"] = manifest(files)
	return verifyStageFiles(directory, files)
}

func verifyStageFiles(directory string, files map[string][]byte) error {
	for name, raw := range files {
		actual, err := readRegular(directory, name, 150_000_000)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, raw) {
			return fmt.Errorf("publication input changed: %s", name)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	if len(entries) != len(files) {
		return fmt.Errorf("unexpected files in publication stage")
	}
	return nil
}

func stageIdentity(policy Policy) map[string][]byte {
	tag := "nhop-xray-" + policy.Version + "-" + strings.TrimPrefix(policy.Label, "nhop-fork-")
	notes := fmt.Sprintf("Maintained Xray %s (%s) for Linux AMD64 and ARM64.\n\n"+
		"Source: %s\nNative protocol acceptance: %s\n\n"+
		"Archives include the exact tested executable, source, LICENSE and module locks. "+
		"SHA256SUMS and per-CPU receipts bind the assets to the reviewed native evidence.\n\n"+
		"MASQUE H3 remains experimental: controlled delay/MTU tests and a rare UDP loss remain unresolved. "+
		"This prerelease does not claim VPS/soak acceptance or change installer pins by itself.\n",
		policy.Version, policy.Label, policy.SourceURL, policy.RunURL)
	return map[string][]byte{"tag": []byte(tag + "\n"), "target": []byte(path.Base(policy.SourceURL) + "\n"), "notes.md": []byte(notes)}
}

func manifest(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var value strings.Builder
	for _, name := range names {
		fmt.Fprintf(&value, "%s  %s\n", hash(files[name]), name)
	}
	return []byte(value.String())
}
