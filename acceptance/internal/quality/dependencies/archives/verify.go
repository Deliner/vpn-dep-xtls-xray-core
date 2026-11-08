package archives

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

func verifyArchive(raw []byte, prefix string, expected map[string]fileIdentity) error {
	if err := validateSourceLinks(expected); err != nil {
		return err
	}
	compressed, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer compressed.Close()
	bounded := &io.LimitedReader{R: compressed, N: 512*1024*1024 + 1}
	reader := tar.NewReader(bounded)
	seen := map[string]bool{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := verifyMember(reader, header, prefix, expected, seen); err != nil {
			return err
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("source archive omits %d tracked files", len(expected)-len(seen))
	}
	// Reading the gzip footer detects truncation and checksum corruption after tar EOF.
	if _, err := io.Copy(tarPadding{}, bounded); err != nil {
		return err
	}
	if bounded.N == 0 {
		return fmt.Errorf("source archive exceeds unpacked size limit")
	}
	return nil
}

type tarPadding struct{}

func (tarPadding) Write(raw []byte) (int, error) {
	for _, value := range raw {
		if value != 0 {
			return 0, fmt.Errorf("unexpected data after source tar end marker")
		}
	}
	return len(raw), nil
}

func verifyMember(reader io.Reader, header *tar.Header, prefix string, expected map[string]fileIdentity, seen map[string]bool) error {
	if header.Typeflag == tar.TypeXGlobalHeader {
		return verifyArchiveComment(header, prefix)
	}
	if header.Typeflag == tar.TypeDir {
		return verifyDirectory(header.Name, prefix, expected, seen)
	}
	if !strings.HasPrefix(header.Name, prefix) {
		return fmt.Errorf("unexpected source archive root %q", header.Name)
	}
	name := strings.TrimPrefix(header.Name, prefix)
	identity, found := expected[name]
	if !found || identity.Gitlink != "" || seen[name] || header.Size < 0 || header.Size > 150_000_000 {
		return fmt.Errorf("unexpected or duplicate source member %q", name)
	}
	if identity.Symlink != "" {
		if err := verifySourceLink(header, identity); err != nil {
			return err
		}
		seen[name] = true
		return nil
	}
	if header.Typeflag != tar.TypeReg {
		return fmt.Errorf("expected regular source member: %s", name)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != identity.SHA256 || (header.Mode&0111 != 0) != identity.Executable {
		return fmt.Errorf("archive member differs from pinned checkout: %s", name)
	}
	seen[name] = true
	return nil
}
