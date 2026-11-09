package release

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"vpn-script/internal/platform/safeio"
)

// Package creates both immutable delivery archives only after native acceptance.
// It owns only the new output directory and never replaces an existing stage.
func Package(policy Policy, collection, output string) (err error) {
	if err := Verify(policy, collection); err != nil {
		return err
	}
	if err := os.Mkdir(output, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(output)
		}
	}()
	for _, arch := range []string{"amd64", "arm64"} {
		members, err := packageMembers(collection, arch)
		if err != nil {
			return err
		}
		raw, err := archiveMembers(members)
		if err != nil {
			return err
		}
		if err := verifyArchive(policy, arch, raw); err != nil {
			return err
		}
		if err := safeio.Create(filepath.Join(output, archiveName(arch)), raw, 0600); err != nil {
			return err
		}
	}
	return sealStage(policy, output)
}

func sealStage(policy Policy, output string) error {
	raw, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return err
	}
	if err := safeio.Create(filepath.Join(output, "policy.json"), append(raw, '\n'), 0600); err != nil {
		return err
	}
	sums, err := stageSums(output)
	if err != nil {
		return err
	}
	return safeio.Create(filepath.Join(output, "SHA256SUMS"), []byte(sums), 0600)
}

func packageMembers(collection, arch string) (map[string][]byte, error) {
	result := map[string][]byte{}
	for name, source := range memberSources() {
		kind, relative, _ := strings.Cut(source, "/")
		root := filepath.Join(collection, "cronet-linux-"+arch)
		if kind == "native" {
			root = filepath.Join(collection, "cronet-native-"+arch)
		}
		path, err := regular(root, relative)
		if err != nil {
			return nil, err
		}
		raw, err := safeio.Read(path, 150_000_000)
		if err != nil {
			return nil, err
		}
		result[name] = raw
	}
	return result, nil
}

func archiveMembers(members map[string][]byte) ([]byte, error) {
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		header.SetMode(0644)
		if name == "libcronet.so" {
			header.SetMode(0755)
		}
		member, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := member.Write(members[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if output.Len() > 150_000_000 {
		return nil, fmt.Errorf("Cronet delivery archive exceeds download limit")
	}
	return output.Bytes(), nil
}
