package singrelease

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"sort"
	"strings"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/runtimerelease"
)

func writeArchive(policy Policy, input Inputs, entries map[string]string, output string) error {
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed := gzip.NewWriter(file)
	defer compressed.Close()
	writer := tar.NewWriter(compressed)
	defer writer.Close()
	prefix := archivePrefix(policy, input.Arch)
	if err := archiveInputs(writer, prefix, input.Candidate, entries); err != nil {
		return err
	}
	if err := archiveMember(writer, prefix+"NHOP-RELEASE.json", releaseMetadata(policy), 0644); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := compressed.Close(); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}

func archiveInputs(writer *tar.Writer, prefix, root string, entries map[string]string) error {
	var names []string
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		member, mode := name, int64(0644)
		switch {
		case name == "bin/sing-box" || name == "bin/libcronet.so":
			member, mode = strings.TrimPrefix(name, "bin/"), 0755
		case strings.HasPrefix(name, "source/"):
		default:
			return fmt.Errorf("unexpected candidate input: %s", name)
		}
		raw, err := runtimerelease.ReadRegular(root, name, 150_000_000)
		if err != nil || safeio.Hash(raw) != entries[name] {
			return fmt.Errorf("candidate changed during packaging: %s", name)
		}
		if err := archiveMember(writer, prefix+member, raw, mode); err != nil {
			return err
		}
	}
	return nil
}

func archiveMember(writer *tar.Writer, name string, raw []byte, mode int64) error {
	header := &tar.Header{Name: name, Mode: mode, Size: int64(len(raw)), Typeflag: tar.TypeReg, Format: tar.FormatPAX}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err := writer.Write(raw)
	return err
}
