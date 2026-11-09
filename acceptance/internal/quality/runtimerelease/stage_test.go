package runtimerelease

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stagedFixture(t *testing.T) (fixture, string) {
	t.Helper()
	f := newFixture(t)
	if err := Prepare(f.policy, f.input, f.output); err != nil {
		t.Fatal(err)
	}
	arm := read(t, filepath.Join(f.output, "receipt.json"))
	var receipt Receipt
	if err := json.Unmarshal(arm, &receipt); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(f.root, "amd64")
	write(t, filepath.Join(other, "receipt.json"), arm)
	if _, _, err := packaged(f.policy, other, "amd64", "receipt.json"); err == nil {
		t.Fatal("architecture mismatch accepted")
	}
	input := f.input
	input.Arch = "amd64"
	changeArch(t, f, input)
	if err := Prepare(f.policy, input, other+"-complete"); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(f.root, "publication")
	if err := Stage(f.policy, other+"-complete", f.output, stage); err != nil {
		t.Fatal(err)
	}
	return f, stage
}

func changeArch(t *testing.T, f fixture, input Inputs) {
	t.Helper()
	for _, name := range []string{"provenance.txt", "audit/provenance.txt"} {
		path := filepath.Join(input.Candidate, name)
		write(t, path, bytes.ReplaceAll(read(t, path), []byte("linux/arm64"), []byte("linux/amd64")))
	}
	host := filepath.Join(input.Native, "host.txt")
	write(t, host, bytes.ReplaceAll(read(t, host), []byte("linux/arm64"), []byte("linux/amd64")))
	files := make(map[string][]byte)
	for _, name := range []string{"xray-SHA256SUMS", "status", "host.txt", "payload/1-hop.json"} {
		files[name] = read(t, filepath.Join(input.Native, name))
	}
	raw := writeInventory(t, input.Native, "evidence.sha256", files)
	target := f.policy.Targets["amd64"]
	target.EvidenceSHA256 = hash(raw)
	f.policy.Targets["amd64"] = target
}

func TestPublicationVerifiesBothArchitecturesAndRejectsExtraFiles(t *testing.T) {
	f, stage := stagedFixture(t)
	if err := VerifyStage(f.policy, stage); err != nil {
		t.Fatal(err)
	}
	if err := Stage(f.policy, f.output, f.output, stage); err == nil {
		t.Fatal("publication replaced existing output or accepted the wrong architecture")
	}
	write(t, filepath.Join(stage, "secret"), []byte("must not be uploaded"))
	if err := VerifyStage(f.policy, stage); err == nil {
		t.Fatal("unreviewed extra upload accepted")
	}
}

func TestRecomputedPackageChecksumCannotAuthorizeDifferentExecutable(t *testing.T) {
	f, stage := stagedFixture(t)
	path := filepath.Join(stage, "Xray-linux-arm64.zip")
	raw := rewriteExecutable(t, read(t, path))
	write(t, path, raw)
	var receipt Receipt
	metadata := filepath.Join(stage, "xray-arm64.json")
	if err := json.Unmarshal(read(t, metadata), &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.SHA256, receipt.Size = hash(raw), len(raw)
	encoded, _ := json.MarshalIndent(receipt, "", "  ")
	write(t, metadata, append(encoded, '\n'))
	if err := VerifyStage(f.policy, stage); err == nil || !strings.Contains(err.Error(), "native-tested executable") {
		t.Fatal("fresh attacker-controlled checksum accepted instead of native identity", err)
	}
}

func rewriteExecutable(t *testing.T, raw []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, member := range reader.File {
		if member.Name == "xray" {
			if err := archiveMember(writer, "xray", []byte("unreviewed executable"), os.FileMode(0755)); err != nil {
				t.Fatal(err)
			}
		} else if err := writer.Copy(member); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
