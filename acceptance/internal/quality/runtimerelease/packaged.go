package runtimerelease

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
)

func packaged(policy Policy, root, arch, receiptName string) (Receipt, []byte, error) {
	var value Receipt
	data, err := readRegular(root, receiptName, 32_768)
	if err != nil {
		return value, nil, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, nil, err
	}
	want := identity(policy, arch)
	want.SHA256, want.Size = value.SHA256, value.Size
	canonical, err := json.MarshalIndent(want, "", "  ")
	if err != nil || value != want || !bytes.Equal(data, append(canonical, '\n')) {
		return value, nil, fmt.Errorf("package receipt differs from reviewed identity")
	}
	raw, err := readRegular(root, value.Archive, 150_000_000)
	if err != nil {
		return value, nil, err
	}
	if hash(raw) != value.SHA256 || len(raw) != value.Size {
		return value, nil, fmt.Errorf("package differs from receipt")
	}
	if err := verifyArchive(policy, arch, raw); err != nil {
		return value, nil, err
	}
	return value, raw, nil
}

func verifyArchive(policy Policy, arch string, raw []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return err
	}
	if len(archive.File) > 20_000 {
		return fmt.Errorf("too many release members")
	}
	entries := make(map[string]string)
	budget := int64(300_000_000)
	for _, member := range archive.File {
		if entries[member.Name] != "" {
			return fmt.Errorf("duplicate release member: %s", member.Name)
		}
		body, err := releaseMember(member, &budget)
		if err != nil {
			return err
		}
		if member.Name == "NHOP-RELEASE.json" {
			metadata, _ := json.MarshalIndent(identity(policy, arch), "", "  ")
			if !bytes.Equal(body, metadata) {
				return fmt.Errorf("embedded release identity changed")
			}
		}
		entries[member.Name] = hash(body)
	}
	if entries["NHOP-RELEASE.json"] == "" || entries["xray"] != policy.Targets[arch].BinarySHA256 {
		return fmt.Errorf("archive lost its native-tested executable or provenance")
	}
	return verifySourceInventory(entries, policy.Targets[arch].InventorySHA256)
}

func releaseMember(member *zip.File, budget *int64) ([]byte, error) {
	name := member.Name
	if !fs.ValidPath(name) || strings.Contains(name, "\\") || !member.Mode().IsRegular() {
		return nil, fmt.Errorf("unsafe release member: %s", name)
	}
	mode := fs.FileMode(0644)
	if name == "xray" {
		mode = 0755
	} else if name != "NHOP-RELEASE.json" && !strings.HasPrefix(name, "source/") {
		return nil, fmt.Errorf("unexpected release member: %s", name)
	}
	if member.Mode().Perm() != mode || member.UncompressedSize64 > uint64(*budget) {
		return nil, fmt.Errorf("invalid release mode or size: %s", name)
	}
	stream, err := member.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	body, err := io.ReadAll(io.LimitReader(stream, *budget+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > *budget || uint64(len(body)) != member.UncompressedSize64 {
		return nil, fmt.Errorf("release content exceeds declared size")
	}
	*budget -= int64(len(body))
	return body, nil
}

func verifySourceInventory(entries map[string]string, expected string) error {
	var names []string
	for name := range entries {
		if strings.HasPrefix(name, "source/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var manifest strings.Builder
	for _, name := range names {
		fmt.Fprintf(&manifest, "%s  %s\n", entries[name], name)
	}
	fmt.Fprintf(&manifest, "%s  bin/xray\n", entries["xray"])
	if hash([]byte(manifest.String())) != expected {
		return fmt.Errorf("packaged sources differ from native-tested inventory")
	}
	return nil
}
