package installation

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/integration/evidence/installation/previous"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

// CacheAssets names every permitted current and retained historical archive.
func CacheAssets(data model.NodeConfig, arch string, upgrade bool) (map[string]model.Asset, error) {
	assets := map[string]model.Asset{}
	for _, program := range engines.Programs(data) {
		asset, exists := program.Runtime.Assets[arch]
		if !exists {
			return nil, fmt.Errorf("missing current runtime architecture")
		}
		assets[program.Name] = asset
		if upgrade {
			historical, err := previous.Runtime(program.Name)
			if err != nil {
				return nil, err
			}
			prior, exists := historical.Assets[arch]
			if !exists {
				return nil, fmt.Errorf("missing previous runtime architecture")
			}
			if prior.SHA256 != asset.SHA256 {
				assets["previous/"+program.Name] = prior
			}
		}
	}
	return assets, nil
}

func CacheSnapshot(directory string, assets map[string]model.Asset) (map[string]string, int64, error) {
	if err := safeio.NoSymlink(directory); err != nil {
		return nil, 0, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, 0, fmt.Errorf("runtime cache is not a private directory")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(assets) == 0 || len(entries) != len(assets) {
		return nil, 0, fmt.Errorf("runtime cache has missing or unexpected archives")
	}
	hashes, seen, size := map[string]string{}, map[string]bool{}, int64(0)
	for name, asset := range assets {
		if !validate.Hash(asset.SHA256) || asset.Size <= 0 || seen[asset.SHA256] {
			return nil, 0, fmt.Errorf("invalid or duplicate runtime archive identity")
		}
		seen[asset.SHA256] = true
		path := filepath.Join(directory, asset.SHA256+".archive")
		if err := cachedArchive(path, asset); err != nil {
			return nil, 0, err
		}
		hashes[name], size = asset.SHA256, size+asset.Size
	}
	return hashes, size, nil
}

func cachedArchive(path string, asset model.Asset) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() != asset.Size {
		return fmt.Errorf("runtime cache entry type, access or size differs")
	}
	sha, err := safeio.FileHash(path)
	if err != nil || sha != asset.SHA256 {
		return fmt.Errorf("runtime cache checksum differs")
	}
	return nil
}
