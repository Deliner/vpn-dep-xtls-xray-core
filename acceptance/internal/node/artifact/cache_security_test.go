package artifact

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/core/model"
)

type cacheTransport func(*http.Request) (*http.Response, error)

func (f cacheTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func cacheResponse(body []byte, size int64) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, ContentLength: size,
		Body: io.NopCloser(bytes.NewReader(body))}
}

func unsafeCacheEntry(t *testing.T, path, kind string) {
	t.Helper()
	var err error
	switch kind {
	case "symlink":
		err = os.Symlink("/dev/null", path)
	case "fifo":
		err = syscall.Mkfifo(path, 0600)
	case "directory":
		err = os.Mkdir(path, 0700)
	}
	require(t, err)
}

func TestCacheRejectsSpecialEntriesWithoutNetworkOrBlocking(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo", "directory"} {
		t.Run(kind, func(t *testing.T) {
			f := cacheFixture(t, 8)
			path := filepath.Join(f.directory, f.asset.SHA256+".archive")
			unsafeCacheEntry(t, path, kind)
			assertCacheRejected(t, f, path)
			if f.calls.Load() != 0 {
				t.Fatal("unsafe cache entry caused network activity")
			}
		})
	}
}

func assertCacheRejected(t *testing.T, f *cacheLab, path string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		raw, err := cached(ctx, f.client, f.asset, f.directory)
		if raw != nil {
			err = nil
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("special cache entry accepted or returned bytes")
		}
	case <-ctx.Done():
		// Unblock a regressed FIFO open so test cleanup does not leave a reader.
		if writer, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			writer.Close()
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("cache read blocked on a special file")
	}
}

func TestCacheRejectsInvalidMetadataEvenWhenBytesArePresent(t *testing.T) {
	f := cacheFixture(t, 8)
	_, err := cached(t.Context(), f.client, f.asset, f.directory)
	require(t, err)
	for _, change := range []func(*model.Asset){
		func(a *model.Asset) { a.URL = "http://unsafe.example.invalid/archive" },
		func(a *model.Asset) { a.URL += "#fragment" },
		func(a *model.Asset) { a.Size = 0 },
		func(a *model.Asset) { a.SHA256 = "../escape" },
	} {
		asset := f.asset
		change(&asset)
		if raw, err := cached(t.Context(), f.client, asset, f.directory); err == nil || raw != nil {
			t.Fatal("cache bypassed pinned asset validation")
		}
	}
	invalid := f.asset
	invalid.SHA256 = "../escape"
	directory := filepath.Join(t.TempDir(), "must-not-create")
	if _, err := cached(t.Context(), f.client, invalid, directory); err == nil {
		t.Fatal("invalid key accepted")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) || f.calls.Load() != 1 {
		t.Fatal("invalid metadata caused filesystem or network effects")
	}
}

func TestCacheDoesNotPublishFailedOrUnverifiedDownloads(t *testing.T) {
	for _, body := range []string{"short", "changed!"} {
		t.Run(body, func(t *testing.T) {
			f := cacheFixture(t, 8)
			f.client.Transport = cacheTransport(func(*http.Request) (*http.Response, error) {
				return cacheResponse([]byte(body), f.asset.Size), nil
			})
			if raw, err := cached(t.Context(), f.client, f.asset, f.directory); err == nil || raw != nil {
				t.Fatal("download integrity failure was hidden")
			}
			entries, err := os.ReadDir(f.directory)
			require(t, err)
			if len(entries) != 0 {
				t.Fatal("unverified download entered cache")
			}
		})
	}
}

func TestCachePreservesCancellationBeforeHitAndAfterDownload(t *testing.T) {
	f := cacheFixture(t, 8)
	_, err := cached(t.Context(), f.client, f.asset, f.directory)
	require(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if raw, err := cached(ctx, f.client, f.asset, f.directory); !errors.Is(err, context.Canceled) || raw != nil {
		t.Fatal("warm cache ignored cancellation", err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()
	f.client.Transport = cacheTransport(func(*http.Request) (*http.Response, error) {
		cancel()
		return cacheResponse(f.body, f.asset.Size), nil
	})
	directory := t.TempDir()
	if raw, err := cached(ctx, f.client, f.asset, directory); !errors.Is(err, context.Canceled) || raw != nil {
		t.Fatal("download cancellation lost before cache publication", err)
	}
	entries, err := os.ReadDir(directory)
	require(t, err)
	if len(entries) != 0 {
		t.Fatal("request cancelled before publication created a cache entry")
	}
}

func TestCacheDoesNotOverwriteSpecialEntryCreatedDuringDownload(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo", "directory"} {
		t.Run(kind, func(t *testing.T) {
			f := cacheFixture(t, 8)
			path := filepath.Join(f.directory, f.asset.SHA256+".archive")
			f.client.Transport = cacheTransport(func(*http.Request) (*http.Response, error) {
				unsafeCacheEntry(t, path, kind)
				return cacheResponse(f.body, f.asset.Size), nil
			})
			if raw, err := cached(t.Context(), f.client, f.asset, f.directory); err == nil || raw != nil {
				t.Fatal("unsafe cache replacement was accepted")
			}
			info, err := os.Lstat(path)
			require(t, err)
			if info.Mode().IsRegular() {
				t.Fatal("special entry was overwritten instead of rejected")
			}
		})
	}
}

func TestCacheDirectorySymlinkCannotRedirectWrites(t *testing.T) {
	f := cacheFixture(t, 8)
	outside, link := t.TempDir(), filepath.Join(f.directory, "linked")
	require(t, os.Symlink(outside, link))
	if raw, err := cached(t.Context(), f.client, f.asset, link); err == nil || raw != nil || f.calls.Load() != 0 {
		t.Fatal("cache directory symlink was followed")
	}
	entries, err := os.ReadDir(outside)
	require(t, err)
	if len(entries) != 0 {
		t.Fatal("cache wrote through a directory symlink")
	}
}
