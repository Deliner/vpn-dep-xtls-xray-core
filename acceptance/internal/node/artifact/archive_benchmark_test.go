package artifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func BenchmarkRuntimeArchiveRetrieval(b *testing.B) {
	for _, size := range []int{1 << 20, 32 << 20} {
		for _, mode := range []string{"download", "warm-cache"} {
			b.Run(fmt.Sprintf("%dMiB/%s", size>>20, mode), func(b *testing.B) { benchmarkRetrieval(b, size, mode) })
		}
	}
}

func benchmarkRetrieval(b *testing.B, size int, mode string) {
	f := cacheFixture(b, size)
	if _, err := retrieve(b, f, mode); err != nil {
		b.Fatal(err)
	}
	f.calls.Store(0)
	f.sent.Store(0)
	b.SetBytes(int64(size))
	for b.Loop() {
		raw, err := retrieve(b, f, mode)
		if err != nil || len(raw) != size {
			b.Fatal("archive retrieval failed", err)
		}
	}
	b.ReportMetric(float64(f.calls.Load())/float64(b.N), "requests/op")
	b.ReportMetric(float64(f.sent.Load())/float64(b.N), "payload-bytes/op")
}

func retrieve(b *testing.B, f *cacheLab, mode string) ([]byte, error) {
	if mode == "warm-cache" {
		return cached(b.Context(), f.client, f.asset, f.directory)
	}
	return download(b.Context(), f.client, f.asset)
}

func BenchmarkRuntimeArchiveExtraction(b *testing.B) {
	for _, size := range []int{1 << 20, 32 << 20} {
		b.Run(fmt.Sprintf("2x%dMiB", size>>20), func(b *testing.B) {
			benchmarkExtraction(b, extractionArchive(b, size), "release/")
		})
	}
}

func BenchmarkPinnedRuntimeExtraction(b *testing.B) {
	path, prefix := os.Getenv("NHOP_BENCH_TAR"), os.Getenv("NHOP_BENCH_TAR_PREFIX")
	if path == "" || prefix == "" {
		b.Skip("set NHOP_BENCH_TAR and NHOP_BENCH_TAR_PREFIX to measure a local runtime archive")
	}
	raw, err := safeio.Read(path, 150_000_000)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("archive SHA256=%s", safeio.Hash(raw))
	benchmarkExtraction(b, raw, prefix)
}

func benchmarkExtraction(b *testing.B, contents []byte, prefix string) {
	directory := b.TempDir()
	for b.Loop() {
		if _, err := ExtractTarFiles(contents, directory, prefix+"sing-box", prefix+"libcronet.so"); err != nil {
			b.Fatal(err)
		}
	}
}

func extractionArchive(b *testing.B, size int) []byte {
	b.Helper()
	contents := make([]byte, size)
	_, _ = rand.New(rand.NewSource(1)).Read(contents)
	var raw bytes.Buffer
	compressed, err := gzip.NewWriterLevel(&raw, gzip.BestSpeed)
	if err != nil {
		b.Fatal(err)
	}
	archive := tar.NewWriter(compressed)
	for _, name := range []string{"sing-box", "libcronet.so"} {
		if err := archive.WriteHeader(&tar.Header{Name: "release/" + name, Mode: 0755, Size: int64(size)}); err != nil {
			b.Fatal(err)
		}
		if _, err := archive.Write(contents); err != nil {
			b.Fatal(err)
		}
	}
	for _, close := range []func() error{archive.Close, compressed.Close} {
		if err := close(); err != nil {
			b.Fatal(err)
		}
	}
	return raw.Bytes()
}
