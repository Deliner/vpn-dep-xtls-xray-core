package safeio

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

const maxHashBytes = 150_000_000

func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// FileHash bounds both file size and working memory, including if a file grows
// after Stat. The final component must be a regular file, never a symlink.
func FileHash(path string) (string, error) {
	file, err := openRegular(path, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > maxHashBytes {
		return "", fmt.Errorf("file exceeds size limit: %s", path)
	}
	hash := sha256.New()
	size, err := io.Copy(hash, io.LimitReader(file, maxHashBytes+1))
	if err != nil {
		return "", err
	}
	if size > maxHashBytes {
		return "", fmt.Errorf("file exceeds size limit: %s", path)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
