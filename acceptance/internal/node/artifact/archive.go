package artifact

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

func Extract(contents []byte, directory string) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return "", fmt.Errorf("invalid runtime archive")
	}
	var selected *zip.File
	for _, member := range archive.File {
		if member.Name == "xray" {
			if selected != nil || !member.Mode().IsRegular() || member.UncompressedSize64 == 0 || member.UncompressedSize64 >= 150_000_000 {
				return "", fmt.Errorf("unsafe runtime archive member")
			}
			selected = member
		}
	}
	if selected == nil {
		return "", fmt.Errorf("runtime archive does not contain xray")
	}
	reader, err := selected.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, int64(selected.UncompressedSize64)+1))
	if err != nil || uint64(len(raw)) != selected.UncompressedSize64 {
		return "", fmt.Errorf("runtime binary extraction failed")
	}
	path := filepath.Join(directory, "xray")
	return path, safeio.Atomic(path, raw, 0755)
}
