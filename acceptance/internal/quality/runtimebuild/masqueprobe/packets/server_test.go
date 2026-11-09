package packets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestServerQueueProbeRejectsUnreviewedSource(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "proxy", "masque")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "server.go")
	original := []byte("package masque\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectServer(root); err == nil {
		t.Fatal("accepted unreviewed server queue source")
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, original) {
		t.Fatal("refused server overlay changed its input")
	}
}
