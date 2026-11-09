package attribution

import (
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

// Package preserves the exact native build source tar and its license texts.
// The caller supplies the tar hash from reviewed build provenance.
func Package(source, expected, output string) (err error) {
	if !validate.Hash(expected) {
		return fmt.Errorf("invalid reviewed source SHA")
	}
	if err := safeio.NoSymlink(source); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 1_000_000_000 {
		return fmt.Errorf("invalid source archive type or size")
	}
	if err := os.Mkdir(output, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(output)
		}
	}()
	notice, err := compressSource(input, expected, filepath.Join(output, "source.tar.gz"))
	if err != nil {
		return err
	}
	if err := safeio.Create(filepath.Join(output, "NOTICE.txt"), notice, 0600); err != nil {
		return err
	}
	return seal(output, expected)
}

func compressSource(input io.Reader, expected, output string) ([]byte, error) {
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	writer := gzip.NewWriter(file)
	defer writer.Close()
	hash := sha256.New()
	stream := io.TeeReader(io.LimitReader(input, 1_000_000_001), io.MultiWriter(writer, hash))
	notice, err := notices(stream)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(io.Discard, stream); err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != expected {
		return nil, fmt.Errorf("source archive differs from native build input")
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	return notice, file.Close()
}

func seal(output, sourceSHA string) error {
	if err := safeio.Create(filepath.Join(output, "source.sha256"), []byte(sourceSHA+"  source.tar\n"), 0600); err != nil {
		return err
	}
	var sums string
	for _, name := range []string{"NOTICE.txt", "source.sha256", "source.tar.gz"} {
		sha, err := safeio.FileHash(filepath.Join(output, name))
		if err != nil {
			return err
		}
		sums += sha + "  " + name + "\n"
	}
	return safeio.Create(filepath.Join(output, "SHA256SUMS"), []byte(sums), 0600)
}
