package attribution

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

// Verify checks a downloaded source archive against reviewed raw-tar provenance,
// then reconstructs its notices instead of trusting a publisher's checksum list.
func Verify(archive, notice, expected string) error {
	if !validate.Hash(expected) {
		return fmt.Errorf("invalid reviewed source SHA")
	}
	if err := safeio.NoSymlink(archive); err != nil {
		return err
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 150_000_000 {
		return fmt.Errorf("invalid compressed source type or size")
	}
	reader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer reader.Close()
	want, err := verifiedNotices(reader, expected)
	if err != nil {
		return err
	}
	actual, err := safeio.Read(notice, 16_000_000)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, want) {
		return fmt.Errorf("published notices differ from complete source texts")
	}
	return nil
}

func verifiedNotices(input io.Reader, expected string) ([]byte, error) {
	hash := sha256.New()
	bounded := &io.LimitedReader{R: input, N: 1_000_000_001}
	stream := io.TeeReader(bounded, hash)
	notice, err := notices(stream)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(io.Discard, stream); err != nil {
		return nil, err
	}
	if bounded.N == 0 || fmt.Sprintf("%x", hash.Sum(nil)) != expected {
		return nil, fmt.Errorf("downloaded source differs from reviewed native input")
	}
	return notice, nil
}
