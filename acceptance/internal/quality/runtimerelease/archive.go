package runtimerelease

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

func archive(output, candidate string, entries map[string]string, metadata []byte) error {
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	if err := archiveInputs(writer, candidate, entries); err != nil {
		return err
	}
	if err := archiveMember(writer, "NHOP-RELEASE.json", metadata, 0644); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}

func archiveInputs(writer *zip.Writer, candidate string, entries map[string]string) error {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := readRegular(candidate, name, 150_000_000)
		if err != nil {
			return err
		}
		if hash(raw) != entries[name] {
			return fmt.Errorf("candidate changed during packaging: %s", name)
		}
		member, mode := name, os.FileMode(0644)
		if name == "bin/xray" {
			member, mode = "xray", 0755
		} else if !strings.HasPrefix(name, "source/") {
			return fmt.Errorf("unexpected candidate member: %s", name)
		}
		if err := archiveMember(writer, member, raw, mode); err != nil {
			return err
		}
	}
	return nil
}

func archiveMember(writer *zip.Writer, name string, raw []byte, mode os.FileMode) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
	member, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(member, bytes.NewReader(raw))
	return err
}
