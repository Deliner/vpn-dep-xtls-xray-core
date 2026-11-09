package singrelease

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
	"vpn-script/internal/platform/safeio"
)

// VerifyArchive reconstructs the native-tested inventory from all downloaded
// source and runtime members. Recomputed outer checksums cannot replace them.
func VerifyArchive(policy Policy, arch, path string) error {
	if err := policy.valid(); err != nil {
		return err
	}
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unreviewed sing-box CPU")
	}
	raw, err := safeio.Read(path, 150_000_000)
	if err != nil {
		return err
	}
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer reader.Close()
	bounded := &io.LimitedReader{R: reader, N: 300_000_001}
	entries, err := archiveHashes(tar.NewReader(bounded), archivePrefix(policy, arch), releaseMetadata(policy))
	if err != nil {
		return err
	}
	if err := zeroPadding(bounded); err != nil || bounded.N == 0 {
		return fmt.Errorf("invalid archive trailer or size: %v", err)
	}
	return verifyPackagedInventory(entries, policy.Targets[arch])
}

func archiveHashes(reader *tar.Reader, prefix string, metadata []byte) (map[string]string, error) {
	entries := map[string]string{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
		name, ok := strings.CutPrefix(header.Name, prefix)
		if !ok || entries[name] != "" || len(entries) >= 20_000 {
			return nil, fmt.Errorf("unexpected or duplicate sing-box member")
		}
		raw, err := archiveBody(reader, header, name)
		if err != nil {
			return nil, err
		}
		if name == "NHOP-RELEASE.json" && !bytes.Equal(raw, metadata) {
			return nil, fmt.Errorf("embedded sing-box policy differs")
		}
		entries[name] = safeio.Hash(raw)
	}
}

func archiveBody(reader io.Reader, header *tar.Header, name string) ([]byte, error) {
	mode := int64(0644)
	if name == "sing-box" || name == "libcronet.so" {
		mode = 0755
	} else if name != "NHOP-RELEASE.json" && !strings.HasPrefix(name, "source/") {
		return nil, fmt.Errorf("unexpected sing-box archive member: %s", name)
	}
	if !fs.ValidPath(name) || strings.Contains(name, "\\") || header.Typeflag != tar.TypeReg || header.Mode != mode || header.Size < 0 || header.Size > 150_000_000 {
		return nil, fmt.Errorf("unsafe sing-box member: %s", name)
	}
	raw, err := io.ReadAll(io.LimitReader(reader, 150_000_001))
	if err != nil || int64(len(raw)) != header.Size {
		return nil, fmt.Errorf("truncated sing-box member: %s", name)
	}
	return raw, nil
}

func zeroPadding(reader io.Reader) error {
	buffer := make([]byte, 8192)
	for {
		n, err := reader.Read(buffer)
		for _, value := range buffer[:n] {
			if value != 0 {
				return fmt.Errorf("unparsed data after tar EOF")
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func verifyPackagedInventory(entries map[string]string, target Target) error {
	if entries["NHOP-RELEASE.json"] == "" || entries["sing-box"] != target.Binary || entries["libcronet.so"] != target.Library {
		return fmt.Errorf("archive lost its tested engine, companion or metadata")
	}
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
	for _, name := range []string{"libcronet.so", "sing-box"} {
		fmt.Fprintf(&manifest, "%s  bin/%s\n", entries[name], name)
	}
	if safeio.Hash([]byte(manifest.String())) != target.Inventory {
		return fmt.Errorf("packaged source/runtime inventory differs from native build")
	}
	return nil
}
