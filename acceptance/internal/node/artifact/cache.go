package artifact

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

// Cached reuses a verified archive, keyed by its pinned content digest. Cache
// files are separate from immutable releases and are never executable inputs
// until their size and complete SHA-256 have been checked again.
func Cached(ctx context.Context, asset model.Asset, directory string) ([]byte, error) {
	client := downloadClient()
	defer client.CloseIdleConnections()
	return cached(ctx, client, asset, directory)
}

func cached(ctx context.Context, client *http.Client, asset model.Asset, directory string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateAsset(asset); err != nil {
		return nil, err
	}
	if err := safeio.PrivateDir(directory); err != nil {
		return nil, err
	}
	path := filepath.Join(directory, asset.SHA256+".archive")
	contents, err := readCache(path, asset)
	if err != nil {
		return nil, err
	}
	if contents == nil {
		contents, err = download(ctx, client, asset)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := cacheEntry(path); err != nil {
			return nil, err
		}
		if err := safeio.Atomic(path, contents, 0600); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return contents, nil
}

func cacheEntry(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("runtime cache requires a regular file: %s", path)
	}
	return info, nil
}

func readCache(path string, asset model.Asset) ([]byte, error) {
	info, err := cacheEntry(path)
	if err != nil || info == nil {
		return nil, err
	}
	if info.Size() != asset.Size {
		return nil, nil
	}
	// Read checks the opened descriptor too: replacing a regular entry with a
	// symlink or special file after Lstat cannot make the read follow or block.
	contents, err := safeio.Read(path, asset.Size)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) != asset.Size || safeio.Hash(contents) != asset.SHA256 {
		return nil, nil
	}
	return contents, nil
}
