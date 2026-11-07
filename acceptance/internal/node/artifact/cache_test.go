package artifact

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

type cacheLab struct {
	client    *http.Client
	server    *httptest.Server
	asset     model.Asset
	directory string
	body      []byte
	calls     atomic.Int64
	sent      atomic.Int64
}

func cacheFixture(t testing.TB, size int) *cacheLab {
	t.Helper()
	f := &cacheLab{directory: t.TempDir(), body: bytes.Repeat([]byte{0x5a}, size)}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.calls.Add(1)
		f.sent.Add(int64(len(f.body)))
		_, _ = w.Write(f.body)
	}))
	f.server = server
	t.Cleanup(server.Close)
	f.client = server.Client()
	f.client.CheckRedirect = httpsRedirect
	t.Cleanup(f.client.CloseIdleConnections)
	f.asset = model.Asset{URL: server.URL + "/runtime", Size: int64(size), SHA256: safeio.Hash(f.body)}
	return f
}

func TestCachedArchiveWorksOfflineAndDoesNotRewriteVerifiedBytes(t *testing.T) {
	f := cacheFixture(t, 4096)
	raw, err := cached(t.Context(), f.client, f.asset, f.directory)
	require(t, err)
	if !bytes.Equal(raw, f.body) || f.calls.Load() != 1 {
		t.Fatal("cold cache did not retrieve the verified archive once")
	}
	path := filepath.Join(f.directory, f.asset.SHA256+".archive")
	before, err := os.Stat(path)
	require(t, err)
	directory, err := os.Stat(f.directory)
	require(t, err)
	if before.Mode().Perm() != 0600 || directory.Mode().Perm() != 0700 {
		t.Fatal("cache permissions are not private")
	}
	f.server.Close()
	raw[0] ^= 0xff
	for range 3 {
		raw, err := cached(t.Context(), f.client, f.asset, f.directory)
		require(t, err)
		if !bytes.Equal(raw, f.body) || f.calls.Load() != 1 {
			t.Fatal("warm cache depended on the network or shared mutable caller bytes")
		}
	}
	after, err := os.Stat(path)
	require(t, err)
	if !os.SameFile(before, after) || before.ModTime() != after.ModTime() {
		t.Fatal("verified cache entry was rewritten")
	}
}

func TestCacheRefreshesChangedTruncatedAndOversizedRegularFiles(t *testing.T) {
	for _, contents := range []string{"changed!", "short", "a larger damaged archive"} {
		t.Run(contents, func(t *testing.T) {
			f := cacheFixture(t, 8)
			path := filepath.Join(f.directory, f.asset.SHA256+".archive")
			require(t, os.WriteFile(path, []byte(contents), 0600))
			raw, err := cached(t.Context(), f.client, f.asset, f.directory)
			require(t, err)
			stored, err := os.ReadFile(path)
			require(t, err)
			if !bytes.Equal(raw, f.body) || !bytes.Equal(stored, f.body) || f.calls.Load() != 1 {
				t.Fatal("damaged cache was trusted or not replaced with verified bytes")
			}
		})
	}
}

func TestCacheSeparatesContentDigestsAndReusesRotatedHTTPSURLs(t *testing.T) {
	first, second := cacheFixture(t, 8), cacheFixture(t, 9)
	for _, f := range []*cacheLab{first, second} {
		_, err := cached(t.Context(), f.client, f.asset, first.directory)
		require(t, err)
		f.server.Close()
		f.asset.URL = "https://unavailable.example.invalid/rotated?token=not-stored"
	}
	for _, f := range []*cacheLab{first, second} {
		raw, err := cached(t.Context(), f.client, f.asset, first.directory)
		require(t, err)
		if !bytes.Equal(raw, f.body) {
			t.Fatal("different pinned archive contents were mixed")
		}
	}
	entries, err := os.ReadDir(first.directory)
	require(t, err)
	if len(entries) != 2 {
		t.Fatal("content-addressed cache did not retain both complete archives")
	}
}

func TestConcurrentCacheWritersPublishOnlyCompleteVerifiedArchives(t *testing.T) {
	f := cacheFixture(t, 1<<20)
	var workers sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		workers.Go(func() {
			raw, err := cached(t.Context(), f.client, f.asset, f.directory)
			if err == nil && !bytes.Equal(raw, f.body) {
				err = errors.New("cache returned incomplete or incorrect archive bytes")
			}
			failures <- err
		})
	}
	workers.Wait()
	close(failures)
	for err := range failures {
		require(t, err)
	}
	entries, err := os.ReadDir(f.directory)
	require(t, err)
	if len(entries) != 1 || entries[0].Name() != f.asset.SHA256+".archive" {
		t.Fatal("cache contains partial files or unexpected metadata")
	}
	raw, err := cached(t.Context(), f.client, f.asset, f.directory)
	require(t, err)
	if !bytes.Equal(raw, f.body) {
		t.Fatal("concurrent cache publication left invalid bytes")
	}
}
