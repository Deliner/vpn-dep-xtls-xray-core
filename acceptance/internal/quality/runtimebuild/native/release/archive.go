package release

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func verifyArchive(policy Policy, arch string, raw []byte) error {
	members, err := readArchive(raw)
	if err != nil {
		return err
	}
	target := policy.Targets[arch]
	for name, sha := range map[string]string{"libcronet.so": target.Library, "source.tsv": policy.Sources,
		"build-evidence.sha256": target.Build, "native-evidence.sha256": target.Native,
		"naive-evidence.sha256": target.Naive, "zlib-evidence.sha256": target.Zlib} {
		if safeio.Hash(members[name]) != sha {
			return fmt.Errorf("unreviewed packaged Cronet member: %s", name)
		}
	}
	for name, source := range memberSources() {
		kind, path, _ := strings.Cut(source, "/")
		if path == "evidence.sha256" || strings.HasSuffix(path, "/evidence.sha256") {
			continue
		}
		entries, err := declaredHashes(members[kind+"-evidence.sha256"])
		if err != nil {
			return err
		}
		if entries[path] != safeio.Hash(members[name]) {
			return fmt.Errorf("packaged member differs from native evidence: %s", name)
		}
	}
	return nil
}

func declaredHashes(raw []byte) (map[string]string, error) {
	result := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		sha, path, ok := strings.Cut(line, "  ")
		path = strings.TrimPrefix(path, "./")
		if !ok || !validate.Hash(sha) || !fs.ValidPath(path) || strings.Contains(path, "\\") || result[path] != "" {
			return nil, fmt.Errorf("invalid packaged native inventory")
		}
		result[path] = sha
	}
	return result, nil
}

func readArchive(raw []byte) (map[string][]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	allowed := memberSources()
	if len(archive.File) != len(allowed) {
		return nil, fmt.Errorf("incomplete Cronet archive")
	}
	result := map[string][]byte{}
	budget := int64(200_000_000)
	for _, member := range archive.File {
		mode := fs.FileMode(0644)
		if member.Name == "libcronet.so" {
			mode = 0755
		}
		if allowed[member.Name] == "" || result[member.Name] != nil || member.Mode() != mode || member.UncompressedSize64 > uint64(budget) {
			return nil, fmt.Errorf("unsafe Cronet archive member: %s", member.Name)
		}
		stream, err := member.Open()
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(stream, budget+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil || int64(len(body)) > budget {
			return nil, fmt.Errorf("invalid Cronet archive data: %s", member.Name)
		}
		budget -= int64(len(body))
		result[member.Name] = body
	}
	return result, nil
}
