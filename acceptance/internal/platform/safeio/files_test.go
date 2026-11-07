//go:build darwin || linux

package safeio_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/platform/safeio"
)

func TestReadRejectsSpecialFiles(t *testing.T) {
	for _, path := range []string{"/dev/null", t.TempDir()} {
		if _, err := safeio.Read(path, 100); err == nil {
			t.Errorf("accepted non-regular file %s", path)
		}
	}
}

func TestReadFIFOIsBoundedWithoutWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.pipe")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := safeio.Read(path, 100); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("accepted FIFO as config")
		}
	case <-time.After(time.Second):
		writer, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			writer.Close()
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("config read blocked waiting for a FIFO writer")
	}
}

func TestReadLimitsAndFinalSymlink(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.json")
	if err := os.WriteFile(path, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if raw, err := safeio.Read(path, 5); err != nil || string(raw) != "12345" {
		t.Fatalf("exact limit: %q, %v", raw, err)
	}
	for _, limit := range []int64{-1, 0, 4, 1<<63 - 1} {
		if _, err := safeio.Read(path, limit); err == nil {
			t.Errorf("accepted invalid/insufficient limit %d", limit)
		}
	}
	link := filepath.Join(directory, "linked.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := safeio.Read(link, 100); err == nil {
		t.Fatal("followed final symlink")
	}
}
