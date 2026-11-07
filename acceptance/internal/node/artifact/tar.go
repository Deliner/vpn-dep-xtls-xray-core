package artifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"vpn-script/internal/platform/safeio"
)

func ExtractTar(contents []byte, directory, member string) (string, error) {
	paths, err := ExtractTarFiles(contents, directory, member)
	return paths[filepath.Base(member)], err
}

// ExtractTarFiles validates every requested runtime member before writing any.
// Publication of the containing private release remains the installer's job.
func ExtractTarFiles(contents []byte, directory string, members ...string) (map[string]string, error) {
	selected, err := tarSelection(members)
	if err != nil {
		return nil, err
	}
	if err := unpackTar(contents, selected, 500_000_000); err != nil {
		return nil, err
	}
	paths := map[string]string{}
	for _, member := range members {
		name := filepath.Base(member)
		path := filepath.Join(directory, name)
		if err := safeio.Atomic(path, selected[member], 0755); err != nil {
			return nil, err
		}
		paths[name] = path
	}
	return paths, nil
}

func tarSelection(members []string) (map[string][]byte, error) {
	if len(members) == 0 || len(members) > 2 {
		return nil, fmt.Errorf("one or two pinned tar members required")
	}
	selected, names := map[string][]byte{}, map[string]bool{}
	for _, member := range members {
		name := filepath.Base(member)
		if (name != "sing-box" && name != "libcronet.so") || names[name] {
			return nil, fmt.Errorf("unexpected or overlapping tar target")
		}
		selected[member], names[name] = nil, true
	}
	return selected, nil
}

func unpackTar(contents []byte, selected map[string][]byte, limit int64) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(contents))
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	limited := &io.LimitedReader{R: gzipReader, N: limit + 1}
	if err := collectTar(tar.NewReader(limited), selected); err != nil {
		return err
	}
	// tar EOF can precede gzip EOF. Drain through the same resource bound so the
	// gzip trailer/checksum is checked before any extracted file is published.
	if _, err := io.Copy(io.Discard, limited); err != nil || limited.N == 0 {
		return fmt.Errorf("invalid or oversized runtime gzip stream")
	}
	for _, contents := range selected {
		if contents == nil {
			return fmt.Errorf("missing pinned runtime member")
		}
	}
	return nil
}

func collectTar(archive *tar.Reader, selected map[string][]byte) error {
	for {
		header, err := archive.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid runtime tar archive")
		}
		previous, wanted := selected[header.Name]
		if !wanted {
			continue
		}
		if previous != nil || header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size >= 150_000_000 {
			return fmt.Errorf("unsafe runtime tar member")
		}
		contents := make([]byte, int(header.Size))
		if _, err := io.ReadFull(archive, contents); err != nil {
			return fmt.Errorf("truncated runtime binary")
		}
		selected[header.Name] = contents
	}
}
