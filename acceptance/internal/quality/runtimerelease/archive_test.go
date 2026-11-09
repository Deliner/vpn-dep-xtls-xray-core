package runtimerelease

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"vpn-script/internal/node/artifact"
)

func TestArchiveExtractsTestedExecutableAndPreservesSource(t *testing.T) {
	f := newFixture(t)
	if err := Prepare(f.policy, f.input, f.output); err != nil {
		t.Fatal(err)
	}
	var receipt Receipt
	if err := json.Unmarshal(read(t, filepath.Join(f.output, "receipt.json")), &receipt); err != nil {
		t.Fatal(err)
	}
	raw := read(t, filepath.Join(f.output, receipt.Archive))
	if receipt.Size != len(raw) || receipt.SHA256 != hash(raw) || receipt.Target != f.policy.Targets["arm64"] {
		t.Fatal("receipt does not identify the delivered bytes")
	}
	path, err := artifact.Extract(raw, t.TempDir())
	if err != nil || !bytes.Equal(read(t, path), read(t, filepath.Join(f.input.Candidate, "bin/xray"))) {
		t.Fatal("installer cannot extract the tested executable", err)
	}
	verifyArchiveMembers(t, raw, f)
}

func verifyArchiveMembers(t *testing.T, raw []byte, f fixture) {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 6 {
		t.Fatal("archive omitted reviewed sources or added unreviewed files")
	}
	for _, member := range archive.File {
		stream, err := member.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(stream)
		stream.Close()
		if err != nil {
			t.Fatal(err)
		}
		if member.Name == "NHOP-RELEASE.json" {
			continue
		}
		name, mode := member.Name, os.FileMode(0644)
		if name == "xray" {
			name, mode = "bin/xray", 0755
		}
		if !bytes.Equal(body, read(t, filepath.Join(f.input.Candidate, name))) || member.Mode().Perm() != mode {
			t.Fatalf("changed member bytes or permissions: %s", member.Name)
		}
	}
}

func TestArchiveIsReproducibleAndNeverOverwritesAnOutput(t *testing.T) {
	f := newFixture(t)
	if err := Prepare(f.policy, f.input, f.output); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(f.root, "repeat")
	if err := Prepare(f.policy, f.input, other); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Xray-linux-arm64.zip", "receipt.json", "SHA256SUMS"} {
		if !bytes.Equal(read(t, filepath.Join(f.output, name)), read(t, filepath.Join(other, name))) {
			t.Fatalf("repeat packaging changed %s", name)
		}
	}
	before := read(t, filepath.Join(f.output, "SHA256SUMS"))
	if err := Prepare(f.policy, f.input, f.output); err == nil {
		t.Fatal("existing output replaced")
	}
	if !bytes.Equal(before, read(t, filepath.Join(f.output, "SHA256SUMS"))) {
		t.Fatal("failed repeat changed the prior release")
	}
}
