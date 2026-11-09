package node

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func digestReader(reader io.Reader, size int64) (string, error) {
	if size < 0 || size > 150_000_000 {
		return "", fmt.Errorf("unexpected archive member size")
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(reader, size+1))
	if err != nil || n != size {
		return "", fmt.Errorf("incomplete archive member: %v", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func tarHashes(reader io.Reader, prefix, gitCommit string) (map[string]string, error) {
	archive := tar.NewReader(reader)
	files := map[string]string{}
	metadata := gitCommit == ""
	for {
		member, err := archive.Next()
		if err == io.EOF {
			if !metadata {
				return nil, fmt.Errorf("Git source archive lacks commit metadata")
			}
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if member.Typeflag == tar.TypeXGlobalHeader {
			if metadata || len(files) != 0 || !gitHeader(member, gitCommit) {
				return nil, fmt.Errorf("unexpected Git archive metadata")
			}
			metadata = true
			continue
		}
		if member.Typeflag == tar.TypeDir {
			continue
		}
		name := strings.TrimPrefix(member.Name, prefix)
		if member.Typeflag != tar.TypeReg || !strings.HasPrefix(member.Name, prefix) ||
			!filepath.IsLocal(name) || filepath.Clean(name) != name || files[name] != "" {
			return nil, fmt.Errorf("unsafe or duplicate archive member: %s", member.Name)
		}
		files[name], err = digestReader(archive, member.Size)
		if err != nil {
			return nil, err
		}
	}
}

func gitHeader(header *tar.Header, commit string) bool {
	return commit != "" && header.Name == "pax_global_header" && len(header.PAXRecords) == 1 &&
		header.PAXRecords["comment"] == commit
}

func gzipHashes(path, prefix string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	files, err := tarHashes(reader, prefix, "")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return nil, err
	}
	return files, nil
}

func zipHash(path, name string) (string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	var selected *zip.File
	for _, file := range archive.File {
		if file.Name == name {
			if selected != nil || !file.Mode().IsRegular() || file.UncompressedSize64 > 150_000_000 {
				return "", fmt.Errorf("unsafe runtime ZIP member")
			}
			selected = file
		}
	}
	if selected == nil {
		return "", fmt.Errorf("runtime ZIP member absent: %s", name)
	}
	reader, err := selected.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	return digestReader(reader, int64(selected.UncompressedSize64))
}
