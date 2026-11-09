package installation

import (
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

func cacheFixture(t *testing.T) (string, map[string]model.Asset) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "cache")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	assets := map[string]model.Asset{}
	for _, name := range []string{"xray", "sing-box", "previous/xray"} {
		raw := []byte("verified-" + name)
		asset := model.Asset{SHA256: safeio.Hash(raw), Size: int64(len(raw))}
		if err := os.WriteFile(filepath.Join(directory, asset.SHA256+".archive"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		assets[name] = asset
	}
	return directory, assets
}

func TestCacheRequiresExactCurrentAndHistoricalArchiveSet(t *testing.T) {
	for _, damage := range []string{"", "missing", "extra", "changed", "linked", "readable", "directory", "duplicate", "invalid-sha"} {
		t.Run(damage, func(t *testing.T) {
			directory, assets := cacheFixture(t)
			path := filepath.Join(directory, assets["previous/xray"].SHA256+".archive")
			damageCache(t, damage, directory, path, assets)
			hashes, size, err := CacheSnapshot(directory, assets)
			if (err == nil) != (damage == "") {
				t.Fatalf("cache evidence mismatch for %s: %v", damage, err)
			}
			if damage == "" && (len(hashes) != 3 || size != 52 || hashes["previous/xray"] != assets["previous/xray"].SHA256) {
				t.Fatal("historical archive was not included in cache evidence", hashes, size)
			}
		})
	}
}

func damageCache(t *testing.T, damage, directory, path string, assets map[string]model.Asset) {
	t.Helper()
	var err error
	switch damage {
	case "missing":
		err = os.Remove(path)
	case "extra":
		err = os.WriteFile(filepath.Join(directory, "extra"), []byte("foreign"), 0600)
	case "changed":
		raw := []byte("verified-previous/xray")
		raw[0] = 'X'
		err = os.WriteFile(path, raw, 0600)
	case "linked", "directory":
		if err = os.Remove(path); err == nil {
			if damage == "linked" {
				err = os.Symlink(assets["xray"].SHA256+".archive", path)
			} else {
				err = os.Mkdir(path, 0700)
			}
		}
	case "readable":
		err = os.Chmod(path, 0644)
	case "duplicate":
		assets["previous/xray"] = assets["xray"]
	case "invalid-sha":
		asset := assets["previous/xray"]
		asset.SHA256 = "../foreign"
		assets["previous/xray"] = asset
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestArchiveCaptureRejectsUnrecordedAndLinkedEntries(t *testing.T) {
	directory, assets := cacheFixture(t)
	expected := map[string]string{}
	for name, asset := range assets {
		expected[name] = asset.SHA256
	}
	if names, err := captureArchiveNames(directory, expected); err != nil || len(names) != 3 {
		t.Fatal("complete reviewed capture rejected", err)
	}
	delete(expected, "previous/xray")
	if _, err := captureArchiveNames(directory, expected); err == nil {
		t.Fatal("unrecorded historical archive accepted")
	}
}
