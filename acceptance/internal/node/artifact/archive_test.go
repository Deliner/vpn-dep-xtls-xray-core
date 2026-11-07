package artifact

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func archive(t *testing.T, mode os.FileMode, duplicate bool) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	count := 1
	if duplicate {
		count = 2
	}
	for range count {
		header := &zip.FileHeader{Name: "xray"}
		header.SetMode(mode)
		file, err := writer.CreateHeader(header)
		require(t, err)
		_, err = file.Write([]byte("sample binary"))
		require(t, err)
	}
	escape, err := writer.Create("../outside")
	require(t, err)
	_, err = escape.Write([]byte("must never be extracted"))
	require(t, err)
	require(t, writer.Close())
	return buffer.Bytes()
}

func TestArchiveExtractsOnlyRegularXray(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	require(t, os.Mkdir(stage, 0700))
	path, err := Extract(archive(t, 0755, false), stage)
	require(t, err)
	if filepath.Base(path) != "xray" {
		t.Fatal("wrong output member")
	}
	if _, err := os.Stat(filepath.Join(root, "outside")); !os.IsNotExist(err) {
		t.Fatal("archive escaped the destination")
	}
	if _, err := Extract(archive(t, os.ModeSymlink|0755, false), stage); err == nil {
		t.Fatal("symlink member accepted")
	}
	if _, err := Extract(archive(t, 0755, true), stage); err == nil {
		t.Fatal("duplicate xray member accepted")
	}
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
